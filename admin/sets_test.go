package admin_test

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/admin"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/sqltest"
)

// base is a shipped set beneath the consumer's own: one transactional
// migration over its own history table.
var base = migrate.Set{
	Name:  "base",
	Table: "base_version",
	Migrations: []migrate.Migration{
		{Version: 1, Name: "base", Up: "CREATE TABLE base_t (x int)", Down: "DROP TABLE base_t", Transactional: true},
	},
}

// twoSets is the migrator every test here administers: base, then the
// consumer's set "app" over the default table.
var twoSets = []migrate.Set{base, {Name: "app", Migrations: set}}

// baseApplied is base's history, clean and complete.
func baseApplied() sqltest.Response {
	return history([]driver.Value{int64(1), "base", false})
}

// bothCurrent is the script of a Status or a Verify over both sets clean
// and complete.
func bothCurrent() []sqltest.Response {
	return []sqltest.Response{exists(true), baseApplied(), exists(true), applied()}
}

// applyingBoth is the script of an Up over an empty database: the lock,
// both history tables read empty, base's migration, app's two, and the
// unlock.
func applyingBoth() []sqltest.Response {
	return []sqltest.Response{
		locked,
		created, history(), created, history(),
		{}, {}, // base 1: up, insert
		{}, {}, // app 1: up, insert
		{}, {}, {}, // app 2: dirty, up, clean
		unlocked,
	}
}

func newSetsService(t *testing.T, opts admin.Options, responses ...sqltest.Response) fixture {
	t.Helper()
	return newSetsFixture(t, twoSets, testDialect{}, opts, responses...)
}

// Status reports one entry per set, in declared order, each with its own
// table, head, latest, and pending versions; one pending set clears Ready.
func TestSets_StatusReportsEverySet(t *testing.T) {
	f := newSetsService(t, admin.Options{},
		exists(true), baseApplied(), exists(true), history([]driver.Value{int64(1), "a", false}))
	st, err := f.service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Ready || f.service.Ready() || len(st.Sets) != 2 {
		t.Fatalf("status = %+v", st)
	}
	b, a := st.Sets[0], st.Sets[1]
	if b.Name != "base" || b.Table != "base_version" || b.Version != 1 || b.Latest != 1 || len(b.Pending) != 0 || !b.Migrations[0].Applied {
		t.Errorf("base = %+v", b)
	}
	if a.Name != "app" || a.Table != migrate.DefaultTable || a.Version != 1 || a.Latest != 2 || !slices.Equal(a.Pending, []int{2}) {
		t.Errorf("app = %+v", a)
	}
}

// Start on an empty database logs each set's pending versions before it
// applies them, then each set's head once current.
func TestSets_StartLogsPendingBySet(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	responses := slices.Concat(
		[]sqltest.Response{exists(false)},                // Verify stops at base
		[]sqltest.Response{exists(false), exists(false)}, // the pending read
		applyingBoth(),
		bothCurrent(),
	)
	f := newSetsService(t, admin.Options{Logger: logger}, responses...)
	if err := f.service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("ready = %v, pending responses = %d", f.service.Ready(), f.rec.Pending())
	}
	for _, want := range []string{
		`msg="schema pending; applying" set=base versions=[1]`,
		`msg="schema pending; applying" set=app versions="[1 2]"`,
		`msg="schema current" set=base version=1`,
		`msg="schema current" set=app version=2`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %s:\n%s", want, buf.String())
		}
	}
}

// Reset reverts the consumer's set before the one it builds on, dropping
// each history table once its set is reverted, then applies both in
// declared order.
func TestSets_ResetRevertsInReverseOrder(t *testing.T) {
	resetting := []sqltest.Response{
		locked,
		created, baseApplied(), created, applied(),
		{}, {}, {}, // app 2: dirty, down, delete
		{}, {}, // app 1: down, delete
		{},     // drop schema_version
		{}, {}, // base 1: down, delete
		{}, // drop base_version
		unlocked,
	}
	responses := slices.Concat(resetting, applyingBoth(), bothCurrent())
	f := newSetsService(t, admin.Options{Seeder: &fakeSeeder{}}, responses...)
	tr, err := f.service.Reset(context.Background(), "default")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if !tr.Schema.Ready || len(tr.Schema.Sets) != 2 || f.rec.Pending() != 0 {
		t.Errorf("transition = %+v, pending = %d", tr, f.rec.Pending())
	}
	var order []string
	for _, e := range f.rec.SQL(sqltest.OpExec) {
		if strings.HasPrefix(e, "DROP") || strings.HasPrefix(e, "CREATE TABLE a") || strings.HasPrefix(e, "CREATE TABLE base_t") {
			order = append(order, e)
		}
	}
	want := []string{
		"DROP INDEX CONCURRENTLY ix", "DROP TABLE a", "DROP TABLE schema_version",
		"DROP TABLE base_t", "DROP TABLE base_version",
		"CREATE TABLE base_t (x int)", "CREATE TABLE a (x int)",
	}
	if !slices.Equal(order, want) {
		t.Errorf("schema statements = %q\nwant %q", order, want)
	}
}

// A dirty consumer set refuses Up; Force on that set by name clears it
// without touching the schema or base's history, and Up then completes.
func TestSets_ForceRepairsTheNamedDirtySet(t *testing.T) {
	dirtyApp := history([]driver.Value{int64(1), "a", false}, []driver.Value{int64(2), "b", true})
	responses := slices.Concat(
		// Up refused at the preflight.
		[]sqltest.Response{locked, created, baseApplied(), created, dirtyApp, unlocked},
		[]sqltest.Response{exists(true), baseApplied(), exists(true), dirtyApp},
		// The status read after the refusal.
		[]sqltest.Response{exists(true), baseApplied(), exists(true), dirtyApp},
		// Force app to 1: create, delete above, mark clean (affects the
		// row, so no insert).
		[]sqltest.Response{locked, {}, {}, {Affected: 1}, unlocked},
		[]sqltest.Response{exists(true), baseApplied(), exists(true), history([]driver.Value{int64(1), "a", false})},
		// Up applies app 2.
		[]sqltest.Response{locked, created, baseApplied(), created, history([]driver.Value{int64(1), "a", false}), {}, {}, {}, unlocked},
		bothCurrent(),
	)
	f := newSetsService(t, admin.Options{}, responses...)
	ctx := context.Background()

	if _, err := f.service.Up(ctx); !errors.Is(err, migrate.ErrDirty) {
		t.Fatalf("Up over a dirty set = %v, want ErrDirty", err)
	} else if se, ok := errors.AsType[*migrate.SetError](err); !ok || se.Set != "app" {
		t.Errorf("Up error = %v, want it to name app", err)
	}
	st, err := f.service.Status(ctx)
	if err != nil || st.Ready || f.service.Ready() {
		t.Fatalf("Status over a dirty set = %+v, %v; ready %v", st, err, f.service.Ready())
	}
	if a := st.Sets[1]; !a.Dirty || a.Version != 2 || len(a.Pending) != 0 || !a.Migrations[0].Applied || a.Migrations[1].Applied {
		t.Errorf("dirty app = %+v", a)
	}
	if b := st.Sets[0]; b.Dirty || b.Version != 1 {
		t.Errorf("base beside a dirty app = %+v", b)
	}
	st, err = f.service.Force(ctx, "app", 1)
	if err != nil {
		t.Fatalf("Force app 1: %v", err)
	}
	if a := st.Sets[1]; a.Dirty || a.Version != 1 || !slices.Equal(a.Pending, []int{2}) {
		t.Errorf("app after Force = %+v", a)
	}
	if st, err = f.service.Up(ctx); err != nil || !st.Ready || !f.service.Ready() {
		t.Fatalf("Up after Force = %+v, %v", st, err)
	}
	if f.rec.Pending() != 0 {
		t.Errorf("pending responses = %d", f.rec.Pending())
	}
	for _, e := range f.rec.SQL(sqltest.OpExec) {
		if strings.Contains(e, "base_version") && !strings.HasPrefix(e, "CREATE TABLE IF NOT EXISTS") {
			t.Errorf("base's history was written: %q", e)
		}
	}
}

// A revert of the set beneath one with applied migrations is refused, and
// nothing runs against the schema.
func TestSets_DownBelowAnAppliedSetIsRefused(t *testing.T) {
	f := newSetsService(t, admin.Options{},
		slices.Concat(
			[]sqltest.Response{locked, created, baseApplied(), created, applied(), unlocked},
			bothCurrent(),
		)...)
	_, err := f.service.Down(context.Background(), "base", 1)
	if !errors.Is(err, migrate.ErrAboveApplied) {
		t.Fatalf("Down base = %v, want ErrAboveApplied", err)
	}
	for _, e := range f.rec.SQL(sqltest.OpExec) {
		if strings.HasPrefix(e, "DROP") || strings.HasPrefix(e, "DELETE") {
			t.Errorf("a refused revert ran %q", e)
		}
	}
	if !f.service.Ready() {
		t.Error("the refusal cleared Ready over an unchanged, current schema")
	}
}

// A verb that names no set, or one the migrator does not run, is refused
// before any I/O, ahead of its other arguments.
func TestSets_UnknownSetRefusedBeforeIO(t *testing.T) {
	f := newSetsService(t, admin.Options{})
	ctx := context.Background()
	cases := map[string]func() (admin.Status, error){
		"down empty":    func() (admin.Status, error) { return f.service.Down(ctx, "", 1) },
		"down nope":     func() (admin.Status, error) { return f.service.Down(ctx, "nope", 1) },
		"steps nope":    func() (admin.Status, error) { return f.service.Steps(ctx, "nope", 0) },
		"force nope":    func() (admin.Status, error) { return f.service.Force(ctx, "nope", 1) },
		"force empty 0": func() (admin.Status, error) { return f.service.Force(ctx, "", 0) },
	}
	for name, verb := range cases {
		if _, err := verb(); !errors.Is(err, admin.ErrUnknownSet) {
			t.Errorf("%s = %v, want ErrUnknownSet", name, err)
		}
	}
	if len(f.rec.Calls()) != 0 {
		t.Errorf("refused verbs reached the database: %v", f.rec.Ops())
	}
}

// With one set pending and another dirty, Start logs nothing as applying:
// Up refuses the run, and startup fails naming the dirty set.
func TestSets_StartOverADirtySetLogsNoApply(t *testing.T) {
	var buf bytes.Buffer
	dirtyApp := history([]driver.Value{int64(1), "a", false}, []driver.Value{int64(2), "b", true})
	f := newSetsService(t, admin.Options{Logger: slog.New(slog.NewTextHandler(&buf, nil))},
		exists(false),                         // Verify stops at base, pending
		exists(false), exists(true), dirtyApp, // the pending read
		locked, created, history(), created, dirtyApp, unlocked, // Up refused
	)
	err := f.service.Start(context.Background())
	if !errors.Is(err, migrate.ErrDirty) || !strings.Contains(err.Error(), `"app"`) {
		t.Fatalf("Start = %v, want the dirty app refusal", err)
	}
	if strings.Contains(buf.String(), "applying") || f.service.Ready() {
		t.Errorf("ready %v, log:\n%s", f.service.Ready(), buf.String())
	}
}

// A history row the set does not carry, as a newer replica leaves it, is
// Status's error, and it clears a Ready an earlier check had set.
func TestSets_StatusOverAnUnknownRowClearsReady(t *testing.T) {
	newer := history([]driver.Value{int64(1), "a", false}, []driver.Value{int64(2), "b", false}, []driver.Value{int64(3), "c", false})
	f := newSetsService(t, admin.Options{},
		slices.Concat(bothCurrent(), []sqltest.Response{exists(true), baseApplied(), exists(true), newer})...)
	ctx := context.Background()
	if err := f.service.Verify(ctx); err != nil || !f.service.Ready() {
		t.Fatalf("Verify = %v, ready %v", err, f.service.Ready())
	}
	_, err := f.service.Status(ctx)
	if !errors.Is(err, migrate.ErrUnknownVersion) {
		t.Fatalf("Status = %v, want ErrUnknownVersion", err)
	}
	if f.service.Ready() {
		t.Error("ready after a history the set does not carry")
	}
}
