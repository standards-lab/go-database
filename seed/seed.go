package seed

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
)

// Step is one ordered unit of a seed run: a seed file bound to the load
// function that inserts its rows. Construct steps with [Table].
type Step struct {
	path string
	load func(ctx context.Context, tx *sql.Tx, decode func(any) error) error
}

// Table binds the seed file at p to a typed load function. The runner
// decodes the file into []T and passes the rows to load inside the step's
// transaction; load owns the insert SQL and its idempotency, where the
// conflict target is known.
func Table[T any](
	p string,
	load func(ctx context.Context, tx *sql.Tx, rows []T) error,
) Step {
	return Step{
		path: p,
		load: func(ctx context.Context, tx *sql.Tx, decode func(any) error) error {
			var rows []T
			if err := decode(&rows); err != nil {
				return fmt.Errorf("decode: %w", err)
			}
			return load(ctx, tx, rows)
		},
	}
}

// Runner executes seed steps against a database, decoding each step's seed
// file with the format that owns its extension.
type Runner struct {
	db      *sql.DB
	logger  *slog.Logger
	formats map[string]Format
}

// New builds a Runner over db, logging each step's outcome to logger. At
// least one Format is required, and no two formats may claim the same
// extension.
func New(db *sql.DB, logger *slog.Logger, formats ...Format) (*Runner, error) {
	if db == nil {
		return nil, fmt.Errorf("seed: nil db")
	}
	if logger == nil {
		return nil, fmt.Errorf("seed: nil logger")
	}
	if len(formats) == 0 {
		return nil, fmt.Errorf("seed: no formats")
	}

	formatMap := make(map[string]Format)
	for _, f := range formats {
		for _, ext := range f.Extensions() {
			if _, dup := formatMap[ext]; dup {
				return nil, fmt.Errorf("seed: duplicate format for extension %q", ext)
			}
			formatMap[ext] = f
		}
	}

	return &Runner{
		db:      db,
		logger:  logger,
		formats: formatMap,
	}, nil
}

// Run executes the steps in order against the seed files in data. Each step
// runs in its own transaction; the first failure stops the run with the
// step's path in the error.
func (r *Runner) Run(ctx context.Context, data fs.FS, steps ...Step) error {
	for _, step := range steps {
		if err := r.execute(ctx, data, step); err != nil {
			return fmt.Errorf("seed %s: %w", step.path, err)
		}
		r.logger.InfoContext(ctx, "seed step applied", "path", step.path)
	}
	return nil
}

func (r *Runner) execute(ctx context.Context, data fs.FS, step Step) error {
	format, ok := r.formats[path.Ext(step.path)]
	if !ok {
		return fmt.Errorf("no format for extension %q", path.Ext(step.path))
	}

	file, err := data.Open(step.path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	decode := func(v any) error { return format.Decode(file, v) }
	if err := step.load(ctx, tx, decode); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback: %w)", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}
