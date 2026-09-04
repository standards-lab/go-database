package admin_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/standards-lab/go-core/lifecycle"
	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/admin"
	"github.com/standards-lab/sqlate"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/query"
	"github.com/standards-lab/sqlate/sqltest"
)

// set is the migration set every test administers: one transactional
// migration and one that runs outside a transaction.
var set = []migrate.Migration{
	{Version: 1, Name: "a", Up: "CREATE TABLE a (x int)", Down: "DROP TABLE a", Transactional: true},
	{Version: 2, Name: "b", Up: "CREATE INDEX CONCURRENTLY ix ON a (x)", Down: "DROP INDEX CONCURRENTLY ix"},
}

var (
	historyCols = []string{"version", "name", "dirty"}
	headCols    = []string{"version", "dirty"}
	locked      = sqltest.Response{}
	created     = sqltest.Response{}
	unlocked    = sqltest.Response{Columns: []string{"unlock"}, Rows: [][]driver.Value{{true}}}
)

func exists(yes bool) sqltest.Response {
	n := int64(0)
	if yes {
		n = 1
	}
	return sqltest.Response{Columns: []string{"count"}, Rows: [][]driver.Value{{n}}}
}

func history(rows ...[]driver.Value) sqltest.Response {
	return sqltest.Response{Columns: historyCols, Rows: rows}
}

func head(version int64, dirty bool) sqltest.Response {
	return sqltest.Response{Columns: headCols, Rows: [][]driver.Value{{version, dirty}}}
}

// applied is the history of the whole set, clean.
func applied() sqltest.Response {
	var rows [][]driver.Value
	for _, m := range set {
		rows = append(rows, []driver.Value{int64(m.Version), m.Name, false})
	}
	return history(rows...)
}

// clean is the script of a Start over a complete, clean history: Verify,
// then Version.
func clean() []sqltest.Response {
	return []sqltest.Response{exists(true), applied(), exists(true), head(2, false)}
}

// applying is the script of an Up over an empty database: the lock, the
// history table, the empty history, each migration's statements, and the
// unlock.
func applying() []sqltest.Response {
	rs := []sqltest.Response{locked, created, history()}
	for _, m := range set {
		if m.Transactional {
			rs = append(rs, sqltest.Response{}, sqltest.Response{}) // up, insert
		} else {
			rs = append(rs, sqltest.Response{}, sqltest.Response{}, sqltest.Response{}) // dirty, up, clean
		}
	}
	return append(rs, unlocked)
}

// testDialect is the stub dialect with the two capabilities an engine
// sub-module supplies: the lock the migrator takes, and the server-version
// statement Diagnose reads.
type testDialect struct{ sqltest.Dialect }

func (testDialect) Lock(ctx context.Context, conn *sql.Conn, name string) error {
	_, err := conn.ExecContext(ctx, "SELECT lock($1)", name)
	return err
}

func (testDialect) Unlock(ctx context.Context, conn *sql.Conn, name string) error {
	var held bool
	if err := conn.QueryRowContext(ctx, "SELECT unlock($1)", name).Scan(&held); err != nil {
		return err
	}
	if !held {
		return errors.New("lock not held")
	}
	return nil
}

func (testDialect) ServerVersion() string { return "SELECT version()" }

// fakeSeeder counts its calls and answers with a fixed count.
type fakeSeeder struct {
	verifyErr error
	verified  int
	seeded    int
}

func (f *fakeSeeder) Verify(context.Context) error { f.verified++; return f.verifyErr }

func (f *fakeSeeder) Seed(context.Context) (admin.Seeded, error) {
	f.seeded++
	return admin.Seeded{"things": 2}, nil
}

// fakeRegistry is one domain's compiled statements.
type fakeRegistry []admin.Entry

func (r fakeRegistry) Registry() []admin.Entry { return r }

// started wraps the fake pool in the provider's lifecycle object, started,
// the way the composition root hands it to the admin service.
func started(t *testing.T, pool *sql.DB) *database.DB {
	t.Helper()
	cfg := database.Config{Name: "test"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatal(err)
	}
	base := database.New(pool, cfg)
	if err := base.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Shutdown(context.Background()) })
	return base
}

type fixture struct {
	service *admin.Service
	rec     *sqltest.Recorder
	seeder  *fakeSeeder
}

// newFixture builds the service over the fake pool with dialect d, the
// migration set, the library's own patterns, a one-statement registry, and
// opts, whose Seeder is replaced by the fixture's fake when it is set.
func newFixture(t *testing.T, d sqlate.Dialect, opts admin.Options, responses ...sqltest.Response) fixture {
	t.Helper()
	pool, rec := sqltest.Open(t, responses...)
	db := sqlate.Wrap(pool, d)
	m, err := migrate.New(db, set, migrate.Options{})
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	catalog := query.MustCatalog(query.Patterns())
	files := fstest.MapFS{"sql/ping.sql": {Data: []byte("--| tier: standard\nSELECT {{n:integer}}")}}
	stmts := catalog.MustCompile(files, "sql", d)
	f := fixture{rec: rec}
	if opts.Seeder != nil {
		f.seeder = &fakeSeeder{}
		opts.Seeder = f.seeder
	}
	opts.Registry = fakeRegistry{{Name: "ping", Statements: stmts}}
	f.service = admin.New(started(t, pool), db, m, catalog, opts)
	return f
}

func newService(t *testing.T, responses ...sqltest.Response) (*admin.Service, *sqltest.Recorder) {
	t.Helper()
	f := newFixture(t, testDialect{}, admin.Options{}, responses...)
	return f.service, f.rec
}

func wantPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, want) {
			t.Errorf("panic = %v, want it to contain %q", r, want)
		}
	}()
	fn()
}

func TestNew_PanicsOnWiringDefects(t *testing.T) {
	pool, _ := sqltest.Open(t)
	db := sqlate.Wrap(pool, testDialect{})
	m, err := migrate.New(db, set, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	catalog := query.MustCatalog(query.Patterns())
	base := started(t, pool)

	wantPanic(t, "nil pool", func() { admin.New(nil, db, m, catalog, admin.Options{}) })
	wantPanic(t, "nil db", func() { admin.New(base, nil, m, catalog, admin.Options{}) })
	wantPanic(t, "nil migrator", func() { admin.New(base, db, nil, catalog, admin.Options{}) })
	wantPanic(t, "nil catalog", func() { admin.New(base, db, m, nil, admin.Options{}) })
	wantPanic(t, "without a Seeder", func() { admin.New(base, db, m, catalog, admin.Options{Seed: true}) })
}

// Start on a complete, clean history verifies and reports ready without
// taking the lock or writing, and verifies the seeder's statements once.
func TestStart_CleanHistoryIsReady(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}}, clean()...)
	if f.service.Ready() {
		t.Fatal("ready before Start")
	}
	if err := f.service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !f.service.Ready() {
		t.Error("not ready after a clean Start")
	}
	if execs := f.rec.SQL(sqltest.OpExec); len(execs) != 0 {
		t.Errorf("Start wrote: %v", execs)
	}
	if f.seeder.verified != 1 || f.seeder.seeded != 0 {
		t.Errorf("seeder verified %d, seeded %d; want 1, 0", f.seeder.verified, f.seeder.seeded)
	}
}

// Start on an empty database applies the whole set under the lock, then
// verifies.
func TestStart_PendingHistoryIsApplied(t *testing.T) {
	responses := append(append([]sqltest.Response{exists(false)}, applying()...), clean()...)
	s, rec := newService(t, responses...)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !s.Ready() || rec.Pending() != 0 {
		t.Errorf("ready = %v, pending responses = %d", s.Ready(), rec.Pending())
	}
	if execs := rec.SQL(sqltest.OpExec); !strings.HasPrefix(execs[0], "SELECT lock") {
		t.Errorf("first exec = %q, want the lock", execs[0])
	}
}

// A dirty history is a state startup cannot correct: Start fails, the
// service stays not ready, and nothing runs against the schema.
func TestStart_DirtyHistoryFailsStartup(t *testing.T) {
	s, rec := newService(t, exists(true),
		history([]driver.Value{int64(1), "a", false}, []driver.Value{int64(2), "b", true}))
	err := s.Start(context.Background())
	if !errors.Is(err, migrate.ErrDirty) {
		t.Fatalf("Start = %v, want ErrDirty", err)
	}
	if s.Ready() || len(rec.SQL(sqltest.OpExec)) != 0 {
		t.Error("a dirty schema reported ready or was written to")
	}
}

// A seeder whose statements do not prepare fails startup and clears
// readiness the schema check had set.
func TestStart_SeederVerifyFailureClearsReady(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}}, clean()...)
	f.seeder.verifyErr = errors.New("seed statement does not prepare")
	err := f.service.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "does not prepare") {
		t.Fatalf("Start = %v, want the seeder's error", err)
	}
	if f.service.Ready() {
		t.Error("ready after the seeder failed to verify")
	}
}

func TestRegister_DeclaresTheSchemaStage(t *testing.T) {
	s, _ := newService(t)
	lc := lifecycle.New()
	s.Register(lc)
	checks := lc.Checks()
	if len(checks) != 1 || checks[0].Name != "schema" || checks[0].Checker != s {
		t.Errorf("checks = %+v, want the service under \"schema\"", checks)
	}
}

// Status reads the head and the pending set, marks each migration, and
// refreshes Ready.
func TestStatus_ReportsHeadAndPending(t *testing.T) {
	s, rec := newService(t,
		exists(true), head(1, false), exists(true), history([]driver.Value{int64(1), "a", false}),
	)
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Version != 1 || st.Dirty || st.Ready || !slices.Equal(st.Pending, []int{2}) {
		t.Errorf("status = %+v", st)
	}
	if len(st.Migrations) != 2 || !st.Migrations[0].Applied || st.Migrations[1].Applied || st.Migrations[1].Transactional {
		t.Errorf("migrations = %+v", st.Migrations)
	}
	if s.Ready() || rec.Pending() != 0 {
		t.Errorf("ready = %v, pending = %d", s.Ready(), rec.Pending())
	}
}

// Verify on a pending schema reports the pending versions and leaves the
// service not ready; on a complete one it sets ready.
func TestVerify_RefreshesReady(t *testing.T) {
	s, _ := newService(t,
		exists(true), history([]driver.Value{int64(1), "a", false}),
		exists(true), applied(),
	)
	err := s.Verify(context.Background())
	if pending, ok := errors.AsType[*migrate.PendingError](err); !ok || !slices.Equal(pending.Versions, []int{2}) {
		t.Fatalf("Verify = %v, want pending [2]", err)
	}
	if s.Ready() {
		t.Error("ready after a pending Verify")
	}
	if err := s.Verify(context.Background()); err != nil || !s.Ready() {
		t.Errorf("second Verify = %v, ready = %v", err, s.Ready())
	}
}

// The verbs reject arguments outside their domain before any I/O.
func TestVerbs_ValidateBeforeIO(t *testing.T) {
	s, rec := newService(t)
	ctx := context.Background()
	cases := map[string]func() (admin.Status, error){
		"down 0":   func() (admin.Status, error) { return s.Down(ctx, 0) },
		"down -1":  func() (admin.Status, error) { return s.Down(ctx, -1) },
		"steps 0":  func() (admin.Status, error) { return s.Steps(ctx, 0) },
		"force -1": func() (admin.Status, error) { return s.Force(ctx, -1) },
	}
	for name, verb := range cases {
		if _, err := verb(); !errors.Is(err, admin.ErrValidation) {
			t.Errorf("%s = %v, want ErrValidation", name, err)
		}
	}
	if len(rec.Calls()) != 0 {
		t.Errorf("rejected verbs reached the database: %v", rec.Ops())
	}
}

// Up applies the set and answers with the refreshed status.
func TestUp_ReturnsRefreshedStatus(t *testing.T) {
	responses := append(applying(), exists(true), head(2, false), exists(true), applied())
	s, rec := newService(t, responses...)
	st, err := s.Up(context.Background())
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if st.Version != 2 || !st.Ready || len(st.Pending) != 0 || !s.Ready() || rec.Pending() != 0 {
		t.Errorf("status = %+v, ready = %v, pending responses = %d", st, s.Ready(), rec.Pending())
	}
}

// Force outside the set is the migrator's error, and the status is still
// read so Ready reflects the schema.
func TestForce_OutsideTheSetIsTheMigratorsError(t *testing.T) {
	s, _ := newService(t)
	_, err := s.Force(context.Background(), 9)
	if !errors.Is(err, migrate.ErrVersionNotFound) {
		t.Errorf("Force 9 = %v, want ErrVersionNotFound", err)
	}
}

// Diagnose pings through the lifecycle object, reads the server's version
// through the dialect's capability, and reports the pool's counters.
func TestDiagnose_ReadsTheVersionThroughTheCapability(t *testing.T) {
	s, rec := newService(t, sqltest.Response{Columns: []string{"version"}, Rows: [][]driver.Value{{"TestDB 1.0"}}})
	d, err := s.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if d.Dialect != "test" || d.ServerVersion != "TestDB 1.0" || d.Pool.MaxOpen != 25 || !slices.Equal(d.Namespaces, []string{"sql"}) {
		t.Errorf("diagnostics = %+v", d)
	}
	if queries := rec.SQL(sqltest.OpQuery); !slices.Equal(queries, []string{"SELECT version()"}) {
		t.Errorf("queries = %v", queries)
	}
}

// A dialect without the capability yields diagnostics with no version and
// no query.
func TestDiagnose_OmitsTheVersionWithoutTheCapability(t *testing.T) {
	f := newFixture(t, sqltest.Dialect{}, admin.Options{})
	d, err := f.service.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if d.ServerVersion != "" || len(f.rec.Calls()) != 0 {
		t.Errorf("diagnostics = %+v, calls = %v", d, f.rec.Calls())
	}
}

// The catalog read is the dump for inspection: the namespace, every
// pattern with its tier and slots, and no I/O.
func TestCatalog_ReadsThePatternsWithoutIO(t *testing.T) {
	s, rec := newService(t)
	c := s.Catalog()
	if !slices.Equal(c.Namespaces, []string{"sql"}) || len(c.Patterns) == 0 {
		t.Fatalf("catalog = %v, %d patterns", c.Namespaces, len(c.Patterns))
	}
	for _, p := range c.Patterns {
		if p.Namespace != "sql" || p.Name == "" || p.Tier == "" || p.Slots == nil || p.Text == "" {
			t.Errorf("entry = %+v", p)
		}
	}
	if len(rec.Calls()) != 0 {
		t.Errorf("the catalog read touched the database: %v", rec.Calls())
	}
}

// The statements read walks the registry: one domain, its statement's
// declarations and parameters, and no I/O.
func TestStatements_ReadTheRegistryWithoutIO(t *testing.T) {
	s, rec := newService(t)
	inv := s.Statements()
	if len(inv.Domains) != 1 || inv.Domains[0].Name != "ping" || len(inv.Domains[0].Statements) != 1 {
		t.Fatalf("inventory = %+v", inv)
	}
	st := inv.Domains[0].Statements[0]
	if st.Name != "ping" || st.Tier != "standard" || !slices.Equal(st.Params, []string{"n"}) || st.Text == "" {
		t.Errorf("ping = %+v", st)
	}
	if len(rec.Calls()) != 0 {
		t.Errorf("the registry read touched the database: %v", rec.Calls())
	}
}

// Without a registry the inventory is empty rather than nil.
func TestStatements_NoRegistryReportsNoDomains(t *testing.T) {
	pool, _ := sqltest.Open(t)
	db := sqlate.Wrap(pool, testDialect{})
	m, err := migrate.New(db, set, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := admin.New(started(t, pool), db, m, query.MustCatalog(query.Patterns()), admin.Options{})
	if inv := s.Statements(); inv.Domains == nil || len(inv.Domains) != 0 {
		t.Errorf("inventory = %+v", inv)
	}
}
