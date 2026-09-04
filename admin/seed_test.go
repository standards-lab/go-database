package admin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/standards-lab/go-database/admin"
)

// Without the switch, Seed refuses before any I/O and the seeder is never
// called.
func TestSeed_DisabledRefuses(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seeder: &fakeSeeder{}})
	n, err := f.service.Seed(context.Background())
	if !errors.Is(err, admin.ErrSeedDisabled) || n != nil {
		t.Fatalf("Seed = %v, %v; want ErrSeedDisabled", n, err)
	}
	if f.seeder.seeded != 0 || len(f.rec.Calls()) != 0 {
		t.Errorf("a disabled seed ran: seeded %d, calls %v", f.seeder.seeded, f.rec.Ops())
	}
}

// Without a seeder, Seed refuses the same way.
func TestSeed_NoSeederRefuses(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.Seed(context.Background()); !errors.Is(err, admin.ErrSeedDisabled) {
		t.Errorf("Seed = %v, want ErrSeedDisabled", err)
	}
}

// With the switch, Seed runs the seeder and reports what it inserted.
func TestSeed_EnabledRunsTheSeeder(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: true, Seeder: &fakeSeeder{}})
	n, err := f.service.Seed(context.Background())
	if err != nil || n["things"] != 2 || f.seeder.seeded != 1 {
		t.Errorf("Seed = %v, %v; seeded %d", n, err, f.seeder.seeded)
	}
}

// With the switch, Start seeds once the schema is current and verified.
func TestStart_SeedsWhenEnabled(t *testing.T) {
	f := newFixture(t, testDialect{}, admin.Options{Seed: true, Seeder: &fakeSeeder{}}, clean()...)
	if err := f.service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !f.service.Ready() || f.seeder.verified != 1 || f.seeder.seeded != 1 || f.rec.Pending() != 0 {
		t.Errorf("ready = %v, verified %d, seeded %d, pending %d", f.service.Ready(), f.seeder.verified, f.seeder.seeded, f.rec.Pending())
	}
}
