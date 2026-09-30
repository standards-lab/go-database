package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/sqlate"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/query"
)

// reverifyInterval is the least time between two of Ready's own schema
// verifications.
const reverifyInterval = 5 * time.Second

var (
	// ErrSeedDisabled reports a seed request with no seeder wired, or with
	// no set named or configured.
	ErrSeedDisabled = errors.New("seeding is disabled")

	// ErrValidation reports a verb argument outside its domain.
	ErrValidation = errors.New("validation failed")

	// ErrUnknownState reports a state name the seeder does not declare.
	ErrUnknownState = errors.New("unknown state")

	// ErrUnknownSet reports a set name the migrator does not run.
	ErrUnknownSet = errors.New("unknown migration set")

	// ErrConflict reports an operation the schema's state refuses: a dirty
	// or pending set, a history its set does not carry, a migration with no
	// down, or a set order the migrator forbids. The migrate sentinel stays
	// in the chain.
	ErrConflict = errors.New("schema conflict")
)

// Seeder is the consumer's seed mechanism over its named sets. Verify
// prepares its statements against the schema; States lists the declared
// names, sorted; Seed applies one set idempotently, even when replicas seed
// it concurrently, and counts what it inserted.
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

// Versioner is the optional dialect capability supplying the statement that
// reads the server's version: one row, one text column.
type Versioner interface {
	ServerVersion() string
}

// Options holds the optional collaborators and the startup seed set.
type Options struct {
	// Seed names the state whose set Start applies and a seed or reset
	// naming no state uses. Empty applies none at startup; a name the
	// seeder does not declare fails Start before it reads the schema.
	Seed string

	// Seeder is the consumer's seed mechanism; nil refuses Seed and Reset.
	Seeder Seeder

	// Registry is the consumer's statements registry. nil means Statements
	// reports no domains.
	Registry Registry

	// Logger receives the startup narrative. nil is silent.
	Logger *slog.Logger
}

// Service is the database admin service.
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
	started  atomic.Bool
	verified atomic.Int64 // unix nanoseconds of Ready's last verification
}

// New builds the service over the pool, its sqlate session, a migrator, and a catalog.
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

// Ready reports whether every set's history is clean and current. Once
// Start has succeeded, a not-ready service verifies the schema itself, at
// most once per five seconds and bounded by the pool's conn_timeout.
func (s *Service) Ready() bool {
	if s.ready.Load() || !s.started.Load() {
		return s.ready.Load()
	}
	now := time.Now().UnixNano()
	last := s.verified.Load()
	if last != 0 && now-last < int64(reverifyInterval) {
		return false
	}
	if !s.verified.CompareAndSwap(last, now) {
		return false // another probe is verifying
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.pool.ConnTimeout())
	defer cancel()
	if s.migrator.Verify(ctx) == nil {
		s.ready.Store(true)
	}
	return s.ready.Load()
}

// Start refuses an undeclared Options.Seed, brings every set to its head
// (logging the pending migrations first), verifies the seeder's statements,
// and applies the configured seed set; the service is ready once all pass.
func (s *Service) Start(ctx context.Context) error {
	if s.seed != "" {
		if err := s.checkState(s.seed); err != nil {
			return err
		}
	}
	err := s.migrator.Verify(ctx)
	if errors.Is(err, migrate.ErrPending) {
		if err := s.logPending(ctx); err != nil {
			return err
		}
		if err := s.migrator.Up(ctx); err != nil {
			return fmt.Errorf("apply: %w", conflict(err))
		}
		err = s.migrator.Verify(ctx)
	}
	if err != nil {
		return conflict(err) // the lifecycle prefixes the service name
	}
	// A clean, complete history puts every set at its latest version.
	for _, l := range s.migrator.Layers() {
		s.logger.InfoContext(ctx, "schema current", "set", l.Name(), "version", latest(l.Migrations()))
	}
	if s.seeder != nil {
		if err := s.seeder.Verify(ctx); err != nil {
			return err
		}
	}
	if s.seed != "" {
		n, err := s.seeder.Seed(ctx, s.seed)
		if err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		s.logger.InfoContext(ctx, "seeded", "state", s.seed, "rows", n)
	}
	s.ready.Store(true)
	s.started.Store(true)
	return nil
}

// logPending logs each set's pending versions, so an operator sees what a
// start applies before it runs. A dirty set logs nothing: Up refuses the
// run, and its error names the set.
func (s *Service) logPending(ctx context.Context) error {
	sets, err := s.migrator.Status(ctx)
	if err != nil {
		return conflict(err)
	}
	for _, st := range sets {
		if st.Dirty {
			return nil
		}
	}
	for _, st := range sets {
		if len(st.Pending) > 0 {
			s.logger.InfoContext(ctx, "schema pending; applying", "set", st.Name, "versions", versions(st.Pending))
		}
	}
	return nil
}

// States lists the state names the seeder declares, sorted.
func (s *Service) States() []string {
	if s.seeder == nil {
		return []string{}
	}
	if states := slices.Clone(s.seeder.States()); states != nil {
		return states
	}
	return []string{}
}

// Seed applies the named set, or the configured one when state is empty,
// over the schema as it stands.
func (s *Service) Seed(ctx context.Context, state string) (Seeded, error) {
	state, err := s.state(state)
	if err != nil {
		return nil, err
	}
	return s.seeder.Seed(ctx, state)
}

// Reset reverts every set, applies every set, and applies the named state's
// seed set, or the configured one when state is empty. A failure leaves the
// schema where its step stopped.
func (s *Service) Reset(ctx context.Context, state string) (Transition, error) {
	state, err := s.state(state)
	if err != nil {
		return Transition{}, err
	}
	if err := s.migrator.Reset(ctx); err != nil {
		_, err = s.after(ctx, fmt.Errorf("revert: %w", conflict(err)))
		return Transition{}, err
	}
	if err := s.migrator.Up(ctx); err != nil {
		_, err = s.after(ctx, fmt.Errorf("apply: %w", conflict(err)))
		return Transition{}, err
	}
	n, err := s.seeder.Seed(ctx, state)
	if err != nil {
		_, err = s.after(ctx, fmt.Errorf("seed: %w", err))
		return Transition{}, err
	}
	st, err := s.Status(ctx)
	if err != nil {
		return Transition{}, err
	}
	s.logger.InfoContext(ctx, "reset", "state", state, "rows", n)
	return Transition{State: state, Schema: st, Seeded: n}, nil
}

// state resolves a seed operation's state name, the configured one when it
// is empty, and refuses it before any I/O: no seeder or no name is
// [ErrSeedDisabled], an undeclared name [ErrUnknownState].
func (s *Service) state(state string) (string, error) {
	if state == "" {
		state = s.seed
	}
	return state, s.checkState(state)
}

// checkState is the refusal every seed operation applies before I/O.
func (s *Service) checkState(state string) error {
	if s.seeder == nil || state == "" {
		return ErrSeedDisabled
	}
	if !slices.Contains(s.seeder.States(), state) {
		return fmt.Errorf("%w: %q", ErrUnknownState, state)
	}
	return nil
}

// Verify checks that every set's history is clean and current and, when it
// is, that the seeder's statements prepare; the error names the set or
// statement at fault.
func (s *Service) Verify(ctx context.Context) error {
	err := s.migrator.Verify(ctx)
	s.settle(err)
	if err != nil {
		return conflict(err)
	}
	if s.seeder != nil {
		return s.seeder.Verify(ctx)
	}
	return nil
}

// Status reads every set's state in declared order.
func (s *Service) Status(ctx context.Context) (Status, error) {
	sets, err := s.migrator.Status(ctx)
	if err != nil {
		s.settle(err)
		return Status{}, conflict(err)
	}
	st := Status{Ready: true, Sets: make([]SetStatus, 0, len(sets))}
	for i, l := range s.migrator.Layers() {
		ss := sets[i]
		out := SetStatus{
			Name: ss.Name, Table: ss.Table, Version: ss.Version, Latest: ss.Latest, Dirty: ss.Dirty,
			Pending: versions(ss.Pending), Migrations: []MigrationInfo{},
		}
		for _, m := range l.Migrations() {
			out.Migrations = append(out.Migrations, MigrationInfo{
				Version: m.Version, Name: m.Name, Transactional: m.Transactional,
				Applied: m.Version < ss.Version || (m.Version == ss.Version && !ss.Dirty),
			})
		}
		if out.Dirty || len(out.Pending) > 0 {
			st.Ready = false
		}
		st.Sets = append(st.Sets, out)
	}
	s.ready.Store(st.Ready)
	return st, nil
}

// Up applies every pending migration of every set, in declared order.
func (s *Service) Up(ctx context.Context) (Status, error) {
	return s.after(ctx, s.migrator.Up(ctx))
}

// Down reverts the named set's n most recent migrations; n must be
// positive.
func (s *Service) Down(ctx context.Context, set string, n int) (Status, error) {
	l, err := s.layer(set)
	if err != nil {
		return Status{}, err
	}
	if n <= 0 {
		return Status{}, fmt.Errorf("%w: steps must be positive", ErrValidation)
	}
	return s.after(ctx, l.Down(ctx, n))
}

// Steps applies the named set's next n pending migrations when n is
// positive, or reverts its last -n applied ones when it is negative; zero
// is rejected.
func (s *Service) Steps(ctx context.Context, set string, n int) (Status, error) {
	l, err := s.layer(set)
	if err != nil {
		return Status{}, err
	}
	if n == 0 {
		return Status{}, fmt.Errorf("%w: steps must be non-zero", ErrValidation)
	}
	return s.after(ctx, l.Steps(ctx, n))
}

// Force sets the named set's history to version, one of its migrations or
// 0 for none, and clears its dirty mark. It touches no schema and checks no
// history first.
func (s *Service) Force(ctx context.Context, set string, version int) (Status, error) {
	l, err := s.layer(set)
	if err != nil {
		return Status{}, err
	}
	if version != 0 && !slices.ContainsFunc(l.Migrations(), func(m migrate.Migration) bool { return m.Version == version }) {
		return Status{}, fmt.Errorf("%w: version %d is not in set %q", ErrValidation, version, set)
	}
	return s.after(ctx, l.Force(ctx, version))
}

// layer resolves a verb's set name, refusing an undeclared one with
// [ErrUnknownSet] before any I/O.
func (s *Service) layer(set string) (migrate.Layer, error) {
	l, ok := s.migrator.Layer(set)
	if !ok {
		return migrate.Layer{}, fmt.Errorf("%w: %q", ErrUnknownSet, set)
	}
	return l, nil
}

// after returns the state following a mutating operation, or the
// operation's error once the status is read, so Ready follows the schema
// the operation left behind.
func (s *Service) after(ctx context.Context, err error) (Status, error) {
	if err != nil {
		_, _ = s.Status(ctx)
		return Status{}, conflict(err)
	}
	return s.Status(ctx)
}

// settle records a determined schema state in the ready flag: true on a
// clean, current schema, false on a dirty, pending, or unrecognized one.
// Any other error, such as a cancelled context or a lost connection,
// determines nothing and leaves the flag.
func (s *Service) settle(err error) {
	switch {
	case err == nil:
		s.ready.Store(true)
	case errors.Is(err, migrate.ErrDirty), errors.Is(err, migrate.ErrPending), errors.Is(err, migrate.ErrUnknownVersion):
		s.ready.Store(false)
	}
}

// conflict wraps [ErrConflict] around a refusal the schema's state causes,
// once.
func conflict(err error) error {
	switch {
	case err == nil, errors.Is(err, ErrConflict):
		return err
	case errors.Is(err, migrate.ErrDirty), errors.Is(err, migrate.ErrPending),
		errors.Is(err, migrate.ErrUnknownVersion), errors.Is(err, migrate.ErrNoDown),
		errors.Is(err, migrate.ErrAboveApplied), errors.Is(err, migrate.ErrBelowPending):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	}
	return err
}

// Catalog reads the pattern catalog: every namespace and every pattern.
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
// inventory.
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

// Diagnose reads the database's health: a timed ping, the server's version
// when the dialect is a [Versioner], and the pool's counters.
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

// versions lists the migrations' versions, empty rather than nil.
func versions(ms []migrate.Migration) []int {
	out := make([]int, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Version)
	}
	return out
}

// latest is the version of a set's last migration, 0 for an empty set.
func latest(ms []migrate.Migration) int {
	if len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].Version
}
