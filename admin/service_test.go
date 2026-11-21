package admin_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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

var set = []migrate.Migration{
	{Version: 1, Name: "a", Up: "CREATE TABLE a (x int)", Down: "DROP TABLE a", Transactional: true},
	{Version: 2, Name: "b", Up: "CREATE INDEX CONCURRENTLY ix ON a (x)", Down: "DROP INDEX CONCURRENTLY ix"},
}

var (
	historyCols = []string{"version", "name", "dirty"}
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

func applied() sqltest.Response {
	var rows [][]driver.Value
	for _, m := range set {
		rows = append(rows, []driver.Value{int64(m.Version), m.Name, false})
	}
	return history(rows...)
}

func clean() []sqltest.Response {
	return []sqltest.Response{exists(true), applied()}
}

func applying() []sqltest.Response {
	rs := []sqltest.Response{locked, created, history()}
	for _, m := range set {
		if m.Transactional {
			rs = append(rs, sqltest.Response{}, sqltest.Response{})
		} else {
			rs = append(rs, sqltest.Response{}, sqltest.Response{}, sqltest.Response{})
		}
	}
	return append(rs, unlocked)
}

func resetting() []sqltest.Response {
	rs := []sqltest.Response{locked, created, applied()}
	for _, s := range slices.Backward(set) {
		if s.Transactional {
			rs = append(rs, sqltest.Response{}, sqltest.Response{})
		} else {
			rs = append(rs, sqltest.Response{}, sqltest.Response{}, sqltest.Response{})
		}
	}
	return append(rs, sqltest.Response{}, unlocked)
}

func current() []sqltest.Response {
	return []sqltest.Response{exists(true), applied()}
}

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

type fakeSeeder struct {
	verifyErr error
	seedErr   error
	states    []string
	verified  int
	seeded    []string
}

func (f *fakeSeeder) Verify(context.Context) error { f.verified++; return f.verifyErr }

func (f *fakeSeeder) States() []string {
	if f.states == nil {
		return []string{"default"}
	}
	return f.states
}

func (f *fakeSeeder) Seed(_ context.Context, state string) (admin.Seeded, error) {
	f.seeded = append(f.seeded, state)
	if f.seedErr != nil {
		return nil, f.seedErr
	}
	return admin.Seeded{"things": 2}, nil
}

var (
	_ lifecycle.Starter          = (*admin.Service)(nil)
	_ lifecycle.ReadinessChecker = (*admin.Service)(nil)
)

type fakeRegistry []admin.Entry

func (r fakeRegistry) Registry() []admin.Entry { return r }

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

func newFixture(t *testing.T, d sqlate.Dialect, opts admin.Options, responses ...sqltest.Response) fixture {
	t.Helper()
	return newSetsFixture(t, []migrate.Set{{Name: "app", Migrations: set}}, d, opts, responses...)
}

func newSetsFixture(t *testing.T, sets []migrate.Set, d sqlate.Dialect, opts admin.Options, responses ...sqltest.Response) fixture {
	t.Helper()
	pool, rec := sqltest.Open(t, responses...)
	db := sqlate.Wrap(pool, d)
	m, err := migrate.New(db, sets, migrate.Options{})
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	catalog := query.MustCatalog(query.Patterns())
	files := fstest.MapFS{"sql/ping.sql": {Data: []byte("--| tier: standard\nSELECT {{n:integer}}")}}
	stmts := catalog.MustCompile(files, "sql", d)
	f := fixture{rec: rec}
	if opts.Seeder != nil {
		f.seeder = opts.Seeder.(*fakeSeeder)
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
	m, err := migrate.New(db, []migrate.Set{{Name: "app", Migrations: set}}, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	catalog := query.MustCatalog(query.Patterns())
	base := started(t, pool)

	wantPanic(t, "nil pool", func() { admin.New(nil, db, m, catalog, admin.Options{}) })
	wantPanic(t, "nil db", func() { admin.New(base, nil, m, catalog, admin.Options{}) })
	wantPanic(t, "nil migrator", func() { admin.New(base, db, nil, catalog, admin.Options{}) })
	wantPanic(t, "nil catalog", func() { admin.New(base, db, m, nil, admin.Options{}) })
	wantPanic(t, "without a Seeder", func() { admin.New(base, db, m, catalog, admin.Options{Seed: "default"}) })
}

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
	if f.seeder.verified != 1 || len(f.seeder.seeded) != 0 {
		t.Errorf("seeder verified %d, seeded %v; want 1, none", f.seeder.verified, f.seeder.seeded)
	}
}

func TestStart_PendingHistoryIsApplied(t *testing.T) {
	responses := slices.Concat([]sqltest.Response{exists(false), exists(false)}, applying(), clean())
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

func TestStatus_ReportsHeadAndPending(t *testing.T) {
	s, rec := newService(t, exists(true), history([]driver.Value{int64(1), "a", false}))
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Ready || len(st.Sets) != 1 {
		t.Fatalf("status = %+v", st)
	}
	app := st.Sets[0]
	if app.Name != "app" || app.Table != migrate.DefaultTable || app.Version != 1 || app.Latest != 2 || app.Dirty || !slices.Equal(app.Pending, []int{2}) {
		t.Errorf("set = %+v", app)
	}
	if len(app.Migrations) != 2 || !app.Migrations[0].Applied || app.Migrations[1].Applied || app.Migrations[1].Transactional {
		t.Errorf("migrations = %+v", app.Migrations)
	}
	if s.Ready() || rec.Pending() != 0 {
		t.Errorf("ready = %v, pending = %d", s.Ready(), rec.Pending())
	}
}

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

func TestVerbs_ValidateBeforeIO(t *testing.T) {
	s, rec := newService(t)
	ctx := context.Background()
	cases := map[string]func() (admin.Status, error){
		"down 0":   func() (admin.Status, error) { return s.Down(ctx, "app", 0) },
		"down -1":  func() (admin.Status, error) { return s.Down(ctx, "app", -1) },
		"steps 0":  func() (admin.Status, error) { return s.Steps(ctx, "app", 0) },
		"force -1": func() (admin.Status, error) { return s.Force(ctx, "app", -1) },
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

func TestUp_ReturnsRefreshedStatus(t *testing.T) {
	responses := append(applying(), current()...)
	s, rec := newService(t, responses...)
	st, err := s.Up(context.Background())
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if !st.Ready || st.Sets[0].Version != 2 || len(st.Sets[0].Pending) != 0 || !s.Ready() || rec.Pending() != 0 {
		t.Errorf("status = %+v, ready = %v, pending responses = %d", st, s.Ready(), rec.Pending())
	}
}

func TestForce_OutsideTheSetIsRefusedBeforeIO(t *testing.T) {
	s, rec := newService(t, clean()...)
	ctx := context.Background()
	if err := s.Verify(ctx); err != nil || !s.Ready() {
		t.Fatalf("Verify = %v, ready %v", err, s.Ready())
	}
	calls := len(rec.Calls())
	_, err := s.Force(ctx, "app", 9)
	if !errors.Is(err, admin.ErrValidation) || !strings.Contains(err.Error(), "9") {
		t.Errorf("Force 9 = %v, want ErrValidation naming the version", err)
	}
	if len(rec.Calls()) != calls || !s.Ready() {
		t.Errorf("calls %d -> %d, ready %v", calls, len(rec.Calls()), s.Ready())
	}
}

func TestSteps_AppliesAndReverts(t *testing.T) {
	one := history([]driver.Value{int64(1), "a", false})
	s, rec := newService(t,
		locked, created, history(), sqltest.Response{}, sqltest.Response{}, unlocked,
		exists(true), one,
		locked, created, applied(), sqltest.Response{}, sqltest.Response{}, sqltest.Response{}, unlocked,
		exists(true), one,
	)
	ctx := context.Background()
	st, err := s.Steps(ctx, "app", 1)
	if err != nil || st.Sets[0].Version != 1 || !slices.Equal(st.Sets[0].Pending, []int{2}) {
		t.Fatalf("Steps +1 = %+v, %v", st, err)
	}
	st, err = s.Steps(ctx, "app", -1)
	if err != nil || st.Sets[0].Version != 1 || st.Ready || s.Ready() {
		t.Fatalf("Steps -1 = %+v, %v; ready %v", st, err, s.Ready())
	}
	if execs := rec.SQL(sqltest.OpExec); !slices.Contains(execs, "CREATE TABLE a (x int)") || !slices.Contains(execs, "DROP INDEX CONCURRENTLY ix") {
		t.Errorf("execs = %q", execs)
	}
	if rec.Pending() != 0 {
		t.Errorf("pending responses = %d", rec.Pending())
	}
}

func TestDown_RevertsAndReportsStatus(t *testing.T) {
	s, rec := newService(t,
		locked, created, applied(), sqltest.Response{}, sqltest.Response{}, sqltest.Response{}, unlocked,
		exists(true), history([]driver.Value{int64(1), "a", false}),
	)
	st, err := s.Down(context.Background(), "app", 1)
	if err != nil || st.Sets[0].Version != 1 || !slices.Equal(st.Sets[0].Pending, []int{2}) || st.Ready {
		t.Fatalf("Down 1 = %+v, %v", st, err)
	}
	if execs := rec.SQL(sqltest.OpExec); !slices.Contains(execs, "DROP INDEX CONCURRENTLY ix") || slices.Contains(execs, "DROP TABLE a") {
		t.Errorf("execs = %q", execs)
	}
	if rec.Pending() != 0 {
		t.Errorf("pending responses = %d", rec.Pending())
	}
}

func TestReady_SurvivesAnAbortedOperation(t *testing.T) {
	s, _ := newService(t, clean()...)
	if err := s.Start(context.Background()); err != nil || !s.Ready() {
		t.Fatalf("Start = %v, ready %v", err, s.Ready())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Status(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Status = %v, want context.Canceled", err)
	}
	if err := s.Verify(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Verify = %v, want context.Canceled", err)
	}
	if _, err := s.Up(ctx); err == nil {
		t.Error("Up under a cancelled context succeeded")
	}
	if !s.Ready() {
		t.Error("an aborted operation cleared Ready over a current schema")
	}
}

func TestReady_ReverifiesACorrectedSchema(t *testing.T) {
	s, rec := newService(t, slices.Concat(
		clean(),
		[]sqltest.Response{exists(true), history([]driver.Value{int64(1), "a", false})},
		current(),
	)...)
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st, err := s.Status(ctx); err != nil || st.Ready {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	if !s.Ready() || rec.Pending() != 0 {
		t.Errorf("ready %v, pending responses %d", s.Ready(), rec.Pending())
	}
	if !s.Ready() {
		t.Error("a verified schema lost Ready")
	}
}

func TestReady_ReverifyIsThrottled(t *testing.T) {
	behind := []sqltest.Response{exists(true), history([]driver.Value{int64(1), "a", false})}
	s, rec := newService(t, slices.Concat(clean(), behind, behind)...)
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.Status(ctx); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if s.Ready() || rec.Pending() != 0 {
		t.Fatalf("ready %v, pending responses %d: the probe did not read the schema", s.Ready(), rec.Pending())
	}
	calls := len(rec.Calls())
	if s.Ready() || len(rec.Calls()) != calls {
		t.Errorf("ready %v, calls %d -> %d", s.Ready(), calls, len(rec.Calls()))
	}
}

func behindService(t *testing.T) *admin.Service {
	t.Helper()
	s, _ := newService(t, slices.Concat(
		clean(), []sqltest.Response{exists(true), history([]driver.Value{int64(1), "a", false})},
	)...)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st, err := s.Status(context.Background()); err != nil || st.Ready {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	return s
}

func TestReady_ConcurrentProbesVerifyOnce(t *testing.T) {
	s := behindService(t)
	var probes atomic.Int32
	admin.SetProbe(s, func(context.Context) error {
		probes.Add(1)
		return migrate.ErrPending
	})
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for range 16 {
		wg.Go(func() {
			<-gate
			_ = s.Ready()
		})
	}
	close(gate)
	wg.Wait()
	if n := probes.Load(); n != 1 {
		t.Errorf("probes = %d, want 1", n)
	}
}

func TestReady_ProbesNeverOverlap(t *testing.T) {
	s := behindService(t)
	var probes atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	admin.SetProbe(s, func(context.Context) error {
		if probes.Add(1) == 1 {
			entered <- struct{}{}
			<-release
		}
		return nil
	})
	done := make(chan bool)
	go func() { done <- s.Ready() }()
	<-entered
	admin.ExpireThrottle(s)
	if s.Ready() || probes.Load() != 1 {
		t.Errorf("a Ready during a probe verified again: probes = %d", probes.Load())
	}
	close(release)
	if !<-done {
		t.Error("the probe's clean verify did not set Ready")
	}
}

func TestReady_ProbeYieldsToAConcurrentFinding(t *testing.T) {
	s, _ := newService(t, slices.Concat(
		clean(),
		[]sqltest.Response{exists(true), history([]driver.Value{int64(1), "a", false})},
		[]sqltest.Response{exists(true), history([]driver.Value{int64(1), "a", false})},
	)...)
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.Status(ctx); err != nil {
		t.Fatalf("Status: %v", err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	admin.SetProbe(s, func(context.Context) error {
		entered <- struct{}{}
		<-release
		return nil
	})
	done := make(chan bool)
	go func() { done <- s.Ready() }()
	<-entered
	if st, err := s.Status(ctx); err != nil || st.Ready {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	close(release)
	if <-done || s.Ready() {
		t.Error("the probe's stale finding overwrote the status found behind")
	}
}

func TestReady_NoVerifyBeforeStart(t *testing.T) {
	s, rec := newService(t)
	if s.Ready() || len(rec.Calls()) != 0 {
		t.Errorf("ready %v, calls %v", s.Ready(), rec.Ops())
	}
}

func TestVerify_SeederFailureLeavesTheSchemaReady(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}}, clean()...)
	f.seeder.verifyErr = errors.New("seed statement does not prepare")
	if err := f.service.Verify(context.Background()); !errors.Is(err, f.seeder.verifyErr) {
		t.Fatalf("Verify = %v, want the seeder's error", err)
	}
	if !f.service.Ready() {
		t.Error("the seeder's failure cleared the schema's Ready")
	}
}

func TestUp_DirtySetIsAConflict(t *testing.T) {
	dirty := history([]driver.Value{int64(1), "a", false}, []driver.Value{int64(2), "b", true})
	s, _ := newService(t, locked, created, dirty, unlocked, exists(true), dirty)
	_, err := s.Up(context.Background())
	if !errors.Is(err, admin.ErrConflict) || !errors.Is(err, migrate.ErrDirty) {
		t.Errorf("Up = %v, want ErrConflict and ErrDirty", err)
	}
	if s.Ready() {
		t.Error("ready over a dirty set")
	}
}

func TestDiagnose_FailingPing(t *testing.T) {
	s, rec := newService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d, err := s.Diagnose(ctx)
	if !errors.Is(err, database.ErrConnectionFailed) || !strings.HasPrefix(err.Error(), "ping: ") {
		t.Fatalf("Diagnose = %v, want the ping's ErrConnectionFailed", err)
	}
	if d.Dialect != "test" || d.ServerVersion != "" || len(rec.Calls()) != 0 {
		t.Errorf("diagnostics = %+v, calls = %v", d, rec.Ops())
	}
}

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

func TestStatements_NoRegistryReportsNoDomains(t *testing.T) {
	pool, _ := sqltest.Open(t)
	db := sqlate.Wrap(pool, testDialect{})
	m, err := migrate.New(db, []migrate.Set{{Name: "app", Migrations: set}}, migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := admin.New(started(t, pool), db, m, query.MustCatalog(query.Patterns()), admin.Options{})
	if inv := s.Statements(); inv.Domains == nil || len(inv.Domains) != 0 {
		t.Errorf("inventory = %+v", inv)
	}
}
