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
// defect: Start fails after the schema check, and nothing is seeded.
func TestStart_UnknownConfiguredSetFailsStartup(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: "nope", Seeder: &fakeSeeder{}}, clean()...)
	err := f.service.Start(context.Background())
	if !errors.Is(err, admin.ErrUnknownState) {
		t.Fatalf("Start = %v, want ErrUnknownState", err)
	}
	if f.seeder.verified != 1 || len(f.seeder.seeded) != 0 {
		t.Errorf("verified %d, seeded %v; want 1, none", f.seeder.verified, f.seeder.seeded)
	}
}

// Reset reverts the applied set under the lock, applies it again, seeds
// the named state's set, and answers with the state, the refreshed
// status, and the rows.
func TestReset_RevertsAppliesAndSeeds(t *testing.T) {
	responses := slices.Concat(reverting(), applying(), current())
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{states: []string{"default", "empty"}}}, responses...)
	tr, err := f.service.Reset(context.Background(), "empty")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if tr.State != "empty" || tr.Schema.Version != 2 || !tr.Schema.Ready || tr.Seeded["things"] != 2 {
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
		"SELECT lock($1)", "DROP INDEX CONCURRENTLY ix", "DROP TABLE a",
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
		locked, created, dirty, unlocked,
		exists(true), head(2, true), exists(true), dirty,
	)
	_, err := f.service.Reset(context.Background(), "default")
	if !errors.Is(err, migrate.ErrDirty) || !strings.HasPrefix(err.Error(), "revert: ") {
		t.Fatalf("Reset = %v, want the revert's ErrDirty", err)
	}
	if len(f.seeder.seeded) != 0 || f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("seeded %v, ready %v, pending %d", f.seeder.seeded, f.service.Ready(), f.rec.Pending())
	}
}
