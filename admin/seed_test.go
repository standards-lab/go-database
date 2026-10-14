package admin_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/admin"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/sqltest"
)

// With no set configured, a seed request that names none refuses before
// any I/O and the seeder is never called.
func TestSeed_NoSetRefuses(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}})
	n, err := f.service.Seed(context.Background(), "")
	if !errors.Is(err, admin.ErrSeedDisabled) || n != nil {
		t.Fatalf("Seed = %v, %v; want ErrSeedDisabled", n, err)
	}
	if len(f.seeder.seeded) != 0 || len(f.rec.Calls()) != 0 {
		t.Errorf("a disabled seed ran: seeded %v, calls %v", f.seeder.seeded, f.rec.Ops())
	}
}

// Without a seeder, every seed operation refuses the same way, and States
// is empty.
func TestSeed_NoSeederRefuses(t *testing.T) {
	s, rec := newService(t)
	ctx := context.Background()
	if _, err := s.Seed(ctx, ""); !errors.Is(err, admin.ErrSeedDisabled) {
		t.Errorf("Seed = %v, want ErrSeedDisabled", err)
	}
	if _, err := s.Seed(ctx, "default"); !errors.Is(err, admin.ErrSeedDisabled) {
		t.Errorf("Seed default = %v, want ErrSeedDisabled", err)
	}
	if _, err := s.Reset(ctx, "default"); !errors.Is(err, admin.ErrSeedDisabled) {
		t.Errorf("Reset = %v, want ErrSeedDisabled", err)
	}
	if states := s.States(); states == nil || len(states) != 0 {
		t.Errorf("States = %#v, want empty", states)
	}
	if len(rec.Calls()) != 0 {
		t.Errorf("refused operations reached the database: %v", rec.Ops())
	}
}

// A name the seeder does not declare refuses Seed and Reset before any
// I/O, and the error names it.
func TestSeed_UnknownStateRefuses(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: "default", Seeder: &fakeSeeder{}})
	ctx := context.Background()
	_, err := f.service.Seed(ctx, "nope")
	if !errors.Is(err, admin.ErrUnknownState) || !strings.Contains(err.Error(), `"nope"`) {
		t.Errorf("Seed nope = %v, want ErrUnknownState naming it", err)
	}
	if _, err := f.service.Reset(ctx, "nope"); !errors.Is(err, admin.ErrUnknownState) {
		t.Errorf("Reset nope = %v, want ErrUnknownState", err)
	}
	if len(f.seeder.seeded) != 0 || len(f.rec.Calls()) != 0 {
		t.Errorf("a refused seed ran: seeded %v, calls %v", f.seeder.seeded, f.rec.Ops())
	}
}

// States is the seeder's list.
func TestStates_ListsTheSeeders(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{states: []string{"default", "empty"}}})
	if got := f.service.States(); !slices.Equal(got, []string{"default", "empty"}) {
		t.Errorf("States = %v", got)
	}
}

// A request that names no set applies the configured one.
func TestSeed_EmptyNameAppliesTheConfiguredSet(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: "default", Seeder: &fakeSeeder{}})
	n, err := f.service.Seed(context.Background(), "")
	if err != nil || n["things"] != 2 || !slices.Equal(f.seeder.seeded, []string{"default"}) {
		t.Errorf("Seed = %v, %v; seeded %v", n, err, f.seeder.seeded)
	}
}

// A named set applies whether or not a set is configured.
func TestSeed_NamedSetAppliesWithoutConfiguration(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{states: []string{"default", "demo"}}})
	n, err := f.service.Seed(context.Background(), "demo")
	if err != nil || n["things"] != 2 || !slices.Equal(f.seeder.seeded, []string{"demo"}) {
		t.Errorf("Seed demo = %v, %v; seeded %v", n, err, f.seeder.seeded)
	}
}

// With a set configured, Start applies it by name once the schema is
// current and verified.
func TestStart_AppliesTheConfiguredSet(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: "default", Seeder: &fakeSeeder{}}, clean()...)
	if err := f.service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !f.service.Ready() || f.seeder.verified != 1 || !slices.Equal(f.seeder.seeded, []string{"default"}) || f.rec.Pending() != 0 {
		t.Errorf("ready = %v, verified %d, seeded %v, pending %d", f.service.Ready(), f.seeder.verified, f.seeder.seeded, f.rec.Pending())
	}
}

// A configured name the seeder does not declare is a configuration
// defect: Start fails before it reads or changes the schema.
func TestStart_UnknownConfiguredSetFailsStartup(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: "nope", Seeder: &fakeSeeder{}})
	err := f.service.Start(context.Background())
	if !errors.Is(err, admin.ErrUnknownState) {
		t.Fatalf("Start = %v, want ErrUnknownState", err)
	}
	if f.seeder.verified != 0 || len(f.seeder.seeded) != 0 || len(f.rec.Calls()) != 0 || f.service.Ready() {
		t.Errorf("verified %d, seeded %v, calls %v, ready %v", f.seeder.verified, f.seeder.seeded, f.rec.Ops(), f.service.Ready())
	}
}

// A startup seed that fails fails Start, and the service is not ready.
func TestStart_SeedFailureFailsStartup(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: "default", Seeder: &fakeSeeder{seedErr: errors.New("seed failed")}}, clean()...)
	err := f.service.Start(context.Background())
	if !errors.Is(err, f.seeder.seedErr) || !strings.HasPrefix(err.Error(), "seed: ") {
		t.Fatalf("Start = %v, want the seed's error", err)
	}
	if f.service.Ready() {
		t.Error("ready after a failed Start")
	}
}

// Reset with no state named brings the database to the configured one,
// and refuses when none is configured.
func TestReset_EmptyNameUsesTheConfiguredSet(t *testing.T) {
	responses := slices.Concat(resetting(), applying(), current())
	f := newFixture(t, testDialect{}, admin.Options{Seed: "default", Seeder: &fakeSeeder{}}, responses...)
	tr, err := f.service.Reset(context.Background(), "")
	if err != nil || tr.State != "default" || !slices.Equal(f.seeder.seeded, []string{"default"}) {
		t.Fatalf("Reset = %+v, %v; seeded %v", tr, err, f.seeder.seeded)
	}

	g := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}})
	if _, err := g.service.Reset(context.Background(), ""); !errors.Is(err, admin.ErrSeedDisabled) {
		t.Errorf("Reset without a configured set = %v, want ErrSeedDisabled", err)
	}
}

// States hands out a copy: changing it leaves the seeder's list alone.
func TestStates_ReturnsACopy(t *testing.T) {
	states := []string{"default", "empty"}
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{states: states}})
	f.service.States()[0] = "changed"
	if states[0] != "default" {
		t.Errorf("seeder's states = %v, want them unchanged", states)
	}
}

// Reset reverts the applied set under the lock and drops its history
// table, applies it again, seeds the named state's set, and answers with
// the state, the refreshed status, and the rows. With no OnReset hook, the
// transition is these steps alone.
func TestReset_RevertsAppliesAndSeeds(t *testing.T) {
	responses := slices.Concat(resetting(), applying(), current())
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{states: []string{"default", "empty"}}}, responses...)
	tr, err := f.service.Reset(context.Background(), "empty")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if tr.State != "empty" || tr.Schema.Sets[0].Version != 2 || !tr.Schema.Ready || tr.Seeded["things"] != 2 {
		t.Errorf("transition = %+v", tr)
	}
	if !slices.Equal(f.seeder.seeded, []string{"empty"}) || !f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("seeded %v, ready %v, pending %d", f.seeder.seeded, f.service.Ready(), f.rec.Pending())
	}
	// The downs run newest first, then the ups oldest first, each pass
	// under its own lock.
	var order []string
	for _, e := range f.rec.SQL(sqltest.OpExec) {
		if strings.HasPrefix(e, "SELECT lock") || strings.HasPrefix(e, "DROP") || strings.HasPrefix(e, "CREATE TABLE a") || strings.HasPrefix(e, "CREATE INDEX") {
			order = append(order, e)
		}
	}
	want := []string{
		"SELECT lock($1)", "DROP INDEX CONCURRENTLY ix", "DROP TABLE a", "DROP TABLE schema_version",
		"SELECT lock($1)", "CREATE TABLE a (x int)", "CREATE INDEX CONCURRENTLY ix ON a (x)",
	}
	if !slices.Equal(order, want) {
		t.Errorf("schema statements = %q\nwant %q", order, want)
	}
}

// A history the transition cannot proceed from stops it at the revert,
// nothing is seeded, and the status is still read so Ready reflects the
// schema left behind.
func TestReset_DirtyHistoryStopsAtTheRevert(t *testing.T) {
	dirty := history([]driver.Value{int64(1), "a", false}, []driver.Value{int64(2), "b", true})
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}},
		exists(true), applied(), // a clean Verify first, so Ready starts true
		locked, created, dirty, unlocked,
		exists(true), dirty,
	)
	if err := f.service.Verify(context.Background()); err != nil || !f.service.Ready() {
		t.Fatalf("Verify = %v, ready %v", err, f.service.Ready())
	}
	_, err := f.service.Reset(context.Background(), "default")
	if !errors.Is(err, migrate.ErrDirty) || !errors.Is(err, admin.ErrConflict) || !strings.HasPrefix(err.Error(), "revert: ") {
		t.Fatalf("Reset = %v, want the revert's ErrDirty", err)
	}
	if len(f.seeder.seeded) != 0 || f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("seeded %v, ready %v, pending %d", f.seeder.seeded, f.service.Ready(), f.rec.Pending())
	}
}

// A transition that fails at the apply seeds nothing, and the status read
// after it finds the schema behind.
func TestReset_FailureAtTheApply(t *testing.T) {
	boom := errors.New("create failed")
	responses := slices.Concat(resetting(),
		[]sqltest.Response{locked, created, history(), {Err: boom}, unlocked},
		[]sqltest.Response{exists(true), history()},
	)
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}}, responses...)
	_, err := f.service.Reset(context.Background(), "default")
	if !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "apply: ") {
		t.Fatalf("Reset = %v, want the apply's error", err)
	}
	if len(f.seeder.seeded) != 0 || f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("seeded %v, ready %v, pending %d", f.seeder.seeded, f.service.Ready(), f.rec.Pending())
	}
}

// A transition that fails at the seed leaves the schema applied and
// current, and Ready reports it so.
func TestReset_FailureAtTheSeed(t *testing.T) {
	boom := errors.New("seed failed")
	responses := slices.Concat(resetting(), applying(), current())
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{seedErr: boom}}, responses...)
	_, err := f.service.Reset(context.Background(), "default")
	if !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "seed: ") {
		t.Fatalf("Reset = %v, want the seed's error", err)
	}
	if !f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("ready %v, pending %d", f.service.Ready(), f.rec.Pending())
	}
}

// hookCall is what an OnReset hook saw when Reset called it.
type hookCall struct {
	state   string
	pending int      // scripted responses not yet consumed
	seeded  []string // the states seeded so far
}

// recordHook returns an OnReset hook that records each call into calls,
// with how far the fixture's script and seeder had run, and answers err.
// It reads *f at the call, so it can be wired before the fixture is built.
func recordHook(f *fixture, calls *[]hookCall, err error) func(context.Context, string) error {
	return func(_ context.Context, state string) error {
		*calls = append(*calls, hookCall{state: state, pending: f.rec.Pending(), seeded: slices.Clone(f.seeder.seeded)})
		return err
	}
}

// Reset calls the hook once, with the state it resolved, after the revert
// and the apply have run and before the seed: at the call, only the status
// read is left of the script, and nothing is seeded.
func TestReset_HookRunsBetweenTheApplyAndTheSeed(t *testing.T) {
	cases := []struct {
		name, configured, request, want string
	}{
		{name: "named state", request: "demo", want: "demo"},
		{name: "configured state", configured: "default", want: "default"},
		{name: "named over configured", configured: "default", request: "demo", want: "demo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []hookCall
			var f fixture
			opts := admin.Options{
				Seed:    tc.configured,
				Seeder:  &fakeSeeder{states: []string{"default", "demo"}},
				OnReset: recordHook(&f, &calls, nil),
			}
			f = newFixture(t, testDialect{}, opts, slices.Concat(resetting(), applying(), current())...)
			tr, err := f.service.Reset(context.Background(), tc.request)
			if err != nil || tr.State != tc.want {
				t.Fatalf("Reset = %+v, %v", tr, err)
			}
			want := []hookCall{{state: tc.want, pending: len(current()), seeded: nil}}
			if !slices.EqualFunc(calls, want, func(a, b hookCall) bool {
				return a.state == b.state && a.pending == b.pending && slices.Equal(a.seeded, b.seeded)
			}) {
				t.Errorf("hook calls = %+v, want %+v", calls, want)
			}
			if !slices.Equal(f.seeder.seeded, []string{tc.want}) || f.rec.Pending() != 0 {
				t.Errorf("seeded %v after the hook, pending %d", f.seeder.seeded, f.rec.Pending())
			}
		})
	}
}

// A hook that fails stops Reset before the seed: the error names the step
// and wraps the hook's, nothing is seeded, and the schema is left applied
// and current, as the status read after it finds.
func TestReset_FailureAtTheHook(t *testing.T) {
	boom := errors.New("hook failed")
	var calls []hookCall
	var f fixture
	opts := admin.Options{
		Seeder:  &fakeSeeder{},
		OnReset: recordHook(&f, &calls, boom),
	}
	f = newFixture(t, testDialect{}, opts, slices.Concat(resetting(), applying(), current())...)
	tr, err := f.service.Reset(context.Background(), "default")
	if !errors.Is(err, boom) || !strings.HasPrefix(err.Error(), "reset hook: ") {
		t.Fatalf("Reset = %+v, %v; want the hook's error", tr, err)
	}
	if len(calls) != 1 || len(f.seeder.seeded) != 0 || !f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("hook calls %d, seeded %v, ready %v, pending %d", len(calls), f.seeder.seeded, f.service.Ready(), f.rec.Pending())
	}
}

// Only Reset calls the hook: Start, Seed, and the schema verbs each run
// to completion without it.
func TestOnReset_OtherVerbsNeverCallIt(t *testing.T) {
	one := history([]driver.Value{int64(1), "a", false})
	responses := slices.Concat(
		clean(),               // Start
		current(),             // Status
		applying(), current(), // Up
		[]sqltest.Response{locked, created, applied(), {}, {}, {}, unlocked, exists(true), one}, // Down 1
		[]sqltest.Response{locked, created, history(), {}, {}, unlocked, exists(true), one},     // Steps +1
		[]sqltest.Response{locked, {}, {}, {Affected: 1}, unlocked, exists(true), one},          // Force 1
	)
	var calls int
	opts := admin.Options{
		Seed:    "default",
		Seeder:  &fakeSeeder{},
		OnReset: func(context.Context, string) error { calls++; return nil },
	}
	f := newFixture(t, testDialect{}, opts, responses...)
	ctx := context.Background()
	verbs := []struct {
		name string
		run  func() error
	}{
		{"Start", func() error { return f.service.Start(ctx) }},
		{"Seed", func() error { _, err := f.service.Seed(ctx, "default"); return err }},
		{"Status", func() error { _, err := f.service.Status(ctx); return err }},
		{"Up", func() error { _, err := f.service.Up(ctx); return err }},
		{"Down", func() error { _, err := f.service.Down(ctx, "app", 1); return err }},
		{"Steps", func() error { _, err := f.service.Steps(ctx, "app", 1); return err }},
		{"Force", func() error { _, err := f.service.Force(ctx, "app", 1); return err }},
	}
	for _, v := range verbs {
		if err := v.run(); err != nil {
			t.Fatalf("%s: %v", v.name, err)
		}
		if calls != 0 {
			t.Fatalf("%s called the hook %d times", v.name, calls)
		}
	}
	if f.rec.Pending() != 0 {
		t.Errorf("pending responses = %d", f.rec.Pending())
	}
}
