package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/standards-lab/go-core/lifecycle"
	"github.com/standards-lab/go-database"
	"github.com/standards-lab/sqlate"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/query"
)

// Stage is the lifecycle stage the schema correction runs in: after the
// pool at stage 0, before the domains verify their statements at stage 2.
const Stage = 1

var (
	// ErrSeedDisabled reports a seed request in an environment whose
	// options do not enable seeding.
	ErrSeedDisabled = errors.New("seeding is disabled")

	// ErrValidation classifies a rejected administrative request: a verb
	// argument outside its domain, refused before any I/O.
	ErrValidation = errors.New("validation failed")
)

// Seeder is the consumer's seed mechanism: Verify prepares its statements
// against the schema, and Seed loads the reference data idempotently and
// counts what it inserted.
type Seeder interface {
	Verify(ctx context.Context) error
	Seed(ctx context.Context) (Seeded, error)
}

// Entry is one domain's compiled statements, registered under its name.
type Entry struct {
	Name       string
	Statements *query.Statements
}

// Registry lists the statements every domain registered, in the order the
// inventory reports them.
type Registry interface {
	Registry() []Entry
}

// Versioner is the optional dialect capability that supplies the statement
// reading the server's version: one row, one text column. Diagnose asserts
// it off the session's dialect and omits the version when it is absent.
type Versioner interface {
	ServerVersion() string
}

// Options holds the collaborators and switches the composition root
// chooses.
type Options struct {
	// Seed enables seeding: at every startup, and on demand. Seeds are
	// development and test tooling; production leaves it off and carries
	// reference data as migrations. Seed without a Seeder is a wiring
	// defect.
	Seed bool

	// Seeder is the consumer's seed mechanism. nil means the consumer has
	// none: Start verifies no seed statements, and Seed refuses.
	Seeder Seeder

	// Registry is the consumer's statements registry. nil means Statements
	// reports no domains.
	Registry Registry

	// Logger receives the startup narrative. nil is silent.
	Logger *slog.Logger
}

// Service is the database admin service. Every operation is a trigger over
// the migrator, the session, the seeder, or the catalog; Start runs the
// same functions the on-demand verbs do. Ready reports a clean, complete
// schema and follows every operation.
type Service struct {
	pool     *database.DB
	db       *sqlate.DB
	migrator *migrate.Migrator
	catalog  *query.Catalog
	seeder   Seeder
	registry Registry
	logger   *slog.Logger
	seed     bool
	ready    atomic.Bool
}

// New builds the service over pool, the lifecycle object it administers;
// db, the sqlate session over the same pool; m, the migrator the consumer
// built over its migration set; and c, the catalog every statement
// compiles against. A nil pool, db, m, or c panics, as does opts.Seed
// without opts.Seeder: each is a wiring defect at the composition root.
func New(pool *database.DB, db *sqlate.DB, m *migrate.Migrator, c *query.Catalog, opts Options) *Service {
	switch {
	case pool == nil:
		panic("admin: nil pool")
	case db == nil:
		panic("admin: nil db")
	case m == nil:
		panic("admin: nil migrator")
	case c == nil:
		panic("admin: nil catalog")
	case opts.Seed && opts.Seeder == nil:
		panic("admin: Seed enabled without a Seeder")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{
		pool: pool, db: db, migrator: m, catalog: c,
		seeder: opts.Seeder, registry: opts.Registry, logger: logger, seed: opts.Seed,
	}
}

// Register declares the schema stage on lc: Start corrects the schema and
// Ready gates readiness on it.
func (s *Service) Register(lc *lifecycle.Coordinator) {
	lc.Add(lifecycle.Service{
		Name:  "schema",
		Stage: Stage,
		Start: s.Start,
		Check: s,
	})
}

// Ready reports whether the history is the set's clean head, as of the
// last operation.
func (s *Service) Ready() bool { return s.ready.Load() }

// Start brings the schema to the set's head: a pending history is applied
// under the migrator's lock; a clean, complete one passes. A state the
// mechanism cannot correct, a dirty row or a history the set does not
// carry, fails startup; an operator resolves it through the verbs (force,
// then up) on a process started against a corrected database, or from
// another replica. The seeder's statements are then verified against the
// schema, and with seeding enabled the seed runs.
func (s *Service) Start(ctx context.Context) error {
	err := s.migrator.Verify(ctx)
	if pending, ok := errors.AsType[*migrate.PendingError](err); ok {
		s.logger.InfoContext(ctx, "schema pending; applying", "versions", pending.Versions)
		if err := s.migrator.Up(ctx); err != nil {
			return fmt.Errorf("apply: %w", err)
		}
		err = s.migrator.Verify(ctx)
	}
	if err != nil {
		return err // the lifecycle prefixes the service name
	}
	s.ready.Store(true)
	v, err := s.migrator.Version(ctx)
	if err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "schema current", "version", v.Version)
	if s.seeder != nil {
		if err := s.seeder.Verify(ctx); err != nil {
			s.ready.Store(false)
			return err
		}
	}
	if s.seed {
		n, err := s.Seed(ctx)
		if err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		s.logger.InfoContext(ctx, "seeded", "rows", n)
	}
	return nil
}

// Seed runs the seeder when this environment enables it; off, or without
// a seeder, the request is refused with [ErrSeedDisabled] before any I/O.
func (s *Service) Seed(ctx context.Context) (Seeded, error) {
	if !s.seed || s.seeder == nil {
		return nil, ErrSeedDisabled
	}
	return s.seeder.Seed(ctx)
}

// Verify checks that the history is the set's clean head and, when it is,
// that the seeder's statements prepare against it; the error names what is
// wrong. Ready follows the result.
func (s *Service) Verify(ctx context.Context) error {
	err := s.migrator.Verify(ctx)
	if err == nil && s.seeder != nil {
		err = s.seeder.Verify(ctx)
	}
	s.ready.Store(err == nil)
	return err
}

// Status reads the schema's state and refreshes Ready from it.
func (s *Service) Status(ctx context.Context) (Status, error) {
	v, err := s.migrator.Version(ctx)
	if err != nil {
		return Status{}, err
	}
	st := Status{Version: v.Version, Dirty: v.Dirty, Pending: []int{}, Migrations: []MigrationInfo{}}
	verr := s.migrator.Verify(ctx)
	if pending, ok := errors.AsType[*migrate.PendingError](verr); ok {
		st.Pending = pending.Versions
	}
	st.Ready = verr == nil
	s.ready.Store(st.Ready)
	for _, m := range s.migrator.Migrations() {
		st.Migrations = append(st.Migrations, MigrationInfo{
			Version: m.Version, Name: m.Name, Transactional: m.Transactional,
			Applied: m.Version < v.Version || (m.Version == v.Version && !v.Dirty),
		})
	}
	return st, nil
}

// Up applies every pending migration and returns the resulting state.
func (s *Service) Up(ctx context.Context) (Status, error) {
	return s.after(ctx, s.migrator.Up(ctx))
}

// Down reverts the n most recent migrations; n must be positive.
func (s *Service) Down(ctx context.Context, n int) (Status, error) {
	if n <= 0 {
		return Status{}, fmt.Errorf("%w: steps must be positive", ErrValidation)
	}
	return s.after(ctx, s.migrator.Down(ctx, n))
}

// Steps applies n pending migrations when n is positive, or reverts -n
// applied ones when it is negative; zero is rejected.
func (s *Service) Steps(ctx context.Context, n int) (Status, error) {
	if n == 0 {
		return Status{}, fmt.Errorf("%w: steps must be non-zero", ErrValidation)
	}
	return s.after(ctx, s.migrator.Steps(ctx, n))
}

// Force sets the history to version, clearing dirty state; 0 empties it.
// It never touches the schema: it exists to clear a dirty row after the
// operator has repaired the schema by hand, and it can just as well
// manufacture one, since a forced-down history re-applies files against
// objects that still exist.
func (s *Service) Force(ctx context.Context, version int) (Status, error) {
	if version < 0 {
		return Status{}, fmt.Errorf("%w: version must not be negative", ErrValidation)
	}
	return s.after(ctx, s.migrator.Force(ctx, version))
}

// after returns the state following a mutating operation, or the
// operation's error. On an error the status is still read, so Ready
// reflects the schema the operation left behind.
func (s *Service) after(ctx context.Context, err error) (Status, error) {
	if err != nil {
		_, _ = s.Status(ctx)
		return Status{}, err
	}
	return s.Status(ctx)
}

// Catalog reads the pattern catalog: every namespace and every pattern, so
// an operator sees a pattern's text, tier, and slots as the library holds
// them. No I/O.
func (s *Service) Catalog() Catalog {
	c := Catalog{Namespaces: s.catalog.Namespaces(), Patterns: []Pattern{}}
	for _, p := range s.catalog.Patterns() {
		slots := p.Slots
		if slots == nil {
			slots = []string{}
		}
		c.Patterns = append(c.Patterns, Pattern{
			Namespace: p.Namespace, Name: p.Name, Tier: string(p.Tier), Native: p.Native, Slots: slots, Text: p.Text,
		})
	}
	return c
}

// Statements reads the statements registry: every domain's compiled
// inventory, the counterpart of Catalog for authored files. No I/O; no
// registry reports no domains.
func (s *Service) Statements() Inventory {
	inv := Inventory{Domains: []DomainStatements{}}
	if s.registry == nil {
		return inv
	}
	for _, r := range s.registry.Registry() {
		d := DomainStatements{Name: r.Name, Statements: []StatementInfo{}}
		for _, st := range r.Statements.Statements() {
			info := StatementInfo{
				Name: st.Name(), Tier: string(st.Tier()), Native: st.Native(), TransactionRequired: st.TransactionRequired(),
				Params: st.Params(), Key: st.Key(), Text: st.Text(),
			}
			if info.Params == nil {
				info.Params = []string{}
			}
			for _, f := range st.Fields() {
				info.Fields = append(info.Fields, f.Name+" "+f.Type)
			}
			d.Statements = append(d.Statements, info)
		}
		inv.Domains = append(inv.Domains, d)
	}
	return inv
}

// Diagnose reads the database's health: the dialect, a timed ping through
// the pool's lifecycle object, the server's version when the dialect
// supplies the statement, and the pool's counters.
func (s *Service) Diagnose(ctx context.Context) (Diagnostics, error) {
	d := Diagnostics{Dialect: s.db.Dialect().Name(), Namespaces: s.catalog.Namespaces()}
	start := time.Now()
	if err := s.pool.Ping(ctx); err != nil {
		return d, fmt.Errorf("ping: %w", err)
	}
	d.Ping = time.Since(start)
	if v, ok := s.db.Dialect().(Versioner); ok {
		version, err := s.serverVersion(ctx, v.ServerVersion())
		if err != nil {
			return d, fmt.Errorf("server version: %w", err)
		}
		d.ServerVersion = version
	}
	st := s.pool.Conn().Stats()
	d.Pool = Pool{
		Open: st.OpenConnections, InUse: st.InUse, Idle: st.Idle, MaxOpen: st.MaxOpenConnections,
		WaitCount: st.WaitCount, WaitDuration: st.WaitDuration,
	}
	return d, nil
}

// serverVersion runs the dialect's version statement and returns its one
// text column; no row is an empty version.
func (s *Service) serverVersion(ctx context.Context, statement string) (string, error) {
	rows, err := s.db.QueryContext(ctx, statement)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	var version string
	if rows.Next() {
		if err := rows.Scan(&version); err != nil {
			return "", s.db.MapError(err)
		}
	}
	return version, s.db.MapError(rows.Err())
}
