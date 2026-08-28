package postgres_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/postgres"
)

func finalizedConfig(t *testing.T) database.Config {
	t.Helper()
	cfg := database.Config{Name: "app", User: "app"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}
	return cfg
}

func TestNew_ConstructsWithoutIO(t *testing.T) {
	db, err := postgres.New(finalizedConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = db.Conn().Close() })

	// Construction never dials; readiness stays false until Start.
	if db.Ready() {
		t.Error("Ready() = true on a freshly constructed database, want false")
	}
}

func TestNew_EmptyUserFallsBackToDriver(t *testing.T) {
	cfg := database.Config{Name: "app"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	// Whether a user is required varies by provider and auth mode, so the
	// config leaves it optional and pgx supplies its OS-user default.
	db, err := postgres.New(cfg)
	if err != nil {
		t.Fatalf("New with no user: %v", err)
	}
	_ = db.Conn().Close()
}

func TestNew_PasswordNeverEntersTheURL(t *testing.T) {
	cfg := database.Config{
		Name:     "app",
		User:     "app",
		Password: `sp ace:sl/ash@at?q&amp'quote`,
	}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	// The password is set as a field on the parsed config, so characters
	// that would break a composed URL cannot: construction succeeds.
	db, err := postgres.New(cfg)
	if err != nil {
		t.Fatalf("New with a hostile password: %v", err)
	}
	_ = db.Conn().Close()
}

func TestNew_RejectsReservedOptions(t *testing.T) {
	reserved := []string{
		"host", "port", "user", "password", "dbname", "database", "connect_timeout",
	}
	for _, key := range reserved {
		t.Run(key, func(t *testing.T) {
			cfg := database.Config{
				Name:    "app",
				Options: map[string]string{key: "x"},
			}
			if err := cfg.Finalize(""); err != nil {
				t.Fatalf("finalize config: %v", err)
			}

			_, err := postgres.New(cfg)
			if err == nil {
				t.Fatalf("New accepted reserved option %q", key)
			}
			if !strings.Contains(err.Error(), "conflicts with a connection field") {
				t.Errorf("error = %v, want the reserved-option message", err)
			}
		})
	}
}

func TestNew_RejectsUnparseableOptions(t *testing.T) {
	cfg := database.Config{
		Name:    "app",
		Options: map[string]string{"sslmode": "bogus"},
	}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	_, err := postgres.New(cfg)
	if err == nil {
		t.Fatal("New accepted an invalid sslmode")
	}
	if !strings.Contains(err.Error(), "parse connection config") {
		t.Errorf("error = %v, want the parse wrap", err)
	}
}

func TestNew_PanicsOnUnfinalizedConfig(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("no panic on an unfinalized config")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "Config not finalized") {
			t.Errorf("panic = %v, want the finalize guidance", r)
		}
	}()
	_, _ = postgres.New(database.Config{Name: "app"})
}

func TestProvider(t *testing.T) {
	if got := string(postgres.Provider); got != "postgres" {
		t.Errorf("Provider = %q, want postgres", got)
	}
}

func TestDialect(t *testing.T) {
	db, err := postgres.New(finalizedConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = db.Conn().Close() })
	d := db.Dialect()

	if got := d.Name(); got != "postgres" {
		t.Errorf("Name() = %q, want postgres", got)
	}
	if got := d.Placeholder(1); got != "$1" {
		t.Errorf("Placeholder(1) = %q, want $1", got)
	}
	if got := d.Placeholder(12); got != "$12" {
		t.Errorf("Placeholder(12) = %q, want $12", got)
	}

	// An error outside the classified constraint classes passes through
	// unchanged: nothing is swallowed, and sql.ErrNoRows flows untouched.
	cause := errors.New("driver failure")
	if got := d.MapError(cause); got != cause {
		t.Errorf("MapError(err) = %v, want the error unchanged", got)
	}
	if got := d.MapError(nil); got != nil {
		t.Errorf("MapError(nil) = %v, want nil", got)
	}
}
