package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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
	// ErrSeedDisabled reports a seed request the service cannot serve: no
	// seeder is wired, or the request names no set and the options
	// configure none.
	ErrSeedDisabled = errors.New("seeding is disabled")

	// ErrValidation classifies a rejected administrative request: a verb
	// argument outside its domain, refused before any I/O.
	ErrValidation = errors.New("validation failed")

	// ErrUnknownState reports a state name the seeder does not declare,
	// refused before any I/O.
	ErrUnknownState = errors.New("unknown state")
)

// Seeder is the consumer's seed mechanism over its named sets. A set is
// the data a deployment or a scenario starts from, declared by the
// consumer under a state name. Verify prepares the seeder's statements
// against the schema; States lists the declared names, sorted, without
// I/O; Seed applies one set idempotently and counts what it inserted.
type Seeder interface {
	Verify(ctx context.Context) error
	States() []string
	Seed(ctx context.Context, state string) (Seeded, error)
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
	// Seed names the state whose set applies at every startup, once the
	// schema is current, and on a seed request that names no set: the way
	// a deployment initializes its data. Empty applies none at startup. A
	// name without a Seeder is a wiring defect; a name the seeder does not
	// declare is a configuration defect that fails startup.
	Seed string

	// Seeder is the consumer's seed mechanism. nil means the consumer has
	// no sets: Start verifies no seed statements, and Seed and Reset
	// refuse.
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
	seed     string
	ready    atomic.Bool
}

// New builds the service over pool, the lifecycle object it administers;
// db, the sqlate session over the same pool; m, the migrator the consumer
// built over its migration set; and c, the catalog every statement
// compiles against. A nil pool, db, m, or c panics, as does an opts.Seed
// name without opts.Seeder: each is a wiring defect at the composition
// root.
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
	case opts.Seed != "" && opts.Seeder == nil:
		panic("admin: Seed set without a Seeder")
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
// schema, and the configured set, when there is one, is applied.
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
	if s.seed != "" {
		n, err := s.Seed(ctx, "")
		if err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		s.logger.InfoContext(ctx, "seeded", "state", s.seed, "rows", n)
	}
	return nil
}

// States lists the state names the seeder declares, sorted; no seeder
// declares none. No I/O.
func (s *Service) States() []string {
	if s.seeder == nil {
		return []string{}
	}
	if states := s.seeder.States(); states != nil {
		return states
	}
	return []string{}
}

// Seed applies the named set idempotently over the schema as it stands,
// or the configured set when state is empty. Without a seeder, or with no
// set named or configured, the request is refused with [ErrSeedDisabled];
// a name the seeder does not declare is refused with [ErrUnknownState];
// both before any I/O.
func (s *Service) Seed(ctx context.Context, state string) (Seeded, error) {
	if state == "" {
		state = s.seed
	}
	if err := s.checkState(state); err != nil {
		return nil, err
	}
	return s.seeder.Seed(ctx, state)
}

// Reset brings the database to the named state: every applied migration
// is reverted, the whole set is applied, and the state's set is seeded,
// each through the same function the verbs and Start run. The revert and
// the apply each hold the migrator's lock; the seed runs outside it, as
// Seed does. The request is refused before any I/O the way Seed refuses;
// a failure at any step leaves the schema where that step stopped, and
// the status is still read so Ready reflects it.
func (s *Service) Reset(ctx context.Context, state string) (Transition, error) {
	if err := s.checkState(state); err != nil {
		return Transition{}, err
	}
	if err := s.migrator.Down(ctx, len(s.migrator.Migrations())); err != nil {
		_, _ = s.Status(ctx)
		return Transition{}, fmt.Errorf("revert: %w", err)
	}
	if err := s.migrator.Up(ctx); err != nil {
		_, _ = s.Status(ctx)
		return Transition{}, fmt.Errorf("apply: %w", err)
	}
	n, err := s.seeder.Seed(ctx, state)
	if err != nil {
		_, _ = s.Status(ctx)
		return Transition{}, fmt.Errorf("seed: %w", err)
	}
	st, err := s.Status(ctx)
	if err != nil {
		return Transition{}, err
	}
	s.logger.InfoContext(ctx, "reset", "state", state, "rows", n)
	return Transition{State: state, Schema: st, Seeded: n}, nil
}

// checkState is the refusal every seed operation applies before I/O: no
// seeder or no name is [ErrSeedDisabled], an undeclared name is
// [ErrUnknownState].
func (s *Service) checkState(state string) error {
	if s.seeder == nil || state == "" {
		return ErrSeedDisabled
	}
	if !slices.Contains(s.seeder.States(), state) {
		return fmt.Errorf("%w: %q", ErrUnknownState, state)
	}
	return nil
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
