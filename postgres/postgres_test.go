package postgres_test

import (
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
	password := `sp ace:sl/ash@at?q&amp'quote`
	cfg := database.Config{Name: "app", User: "app", Password: password}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	// The password is set as a field on the parsed config, so characters
	// that would break a composed URL cannot, and the URL never holds it.
	connCfg, err := postgres.ConnConfig(cfg)
	if err != nil {
		t.Fatalf("connConfig with a hostile password: %v", err)
	}
	if connCfg.Password != password {
		t.Errorf("Password = %q, want %q", connCfg.Password, password)
	}
	if dsn := connCfg.ConnString(); strings.Contains(dsn, "sp ace") || strings.Contains(dsn, "quote") || strings.Contains(dsn, "password") {
		t.Errorf("connection string %q carries the password", dsn)
	}
}

// A host that is a path is a Unix-socket directory: it rides the query,
// not the URL's authority, and the port names the socket file.
func TestNew_UnixSocketHost(t *testing.T) {
	cfg := database.Config{Name: "app", User: "app", Host: "/var/run/postgresql", Port: new(5433)}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	connCfg, err := postgres.ConnConfig(cfg)
	if err != nil {
		t.Fatalf("connConfig over a socket directory: %v", err)
	}
	if connCfg.Host != "/var/run/postgresql" || connCfg.Port != 5433 || connCfg.Database != "app" {
		t.Errorf("host %q, port %d, database %q", connCfg.Host, connCfg.Port, connCfg.Database)
	}
	db, err := postgres.New(cfg)
	if err != nil {
		t.Fatalf("New over a socket directory: %v", err)
	}
	_ = db.Conn().Close()
}

// A TCP host keeps the host and port in the URL's authority.
func TestNew_TCPHost(t *testing.T) {
	cfg := database.Config{Name: "app", User: "app", Host: "db.internal"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	connCfg, err := postgres.ConnConfig(cfg)
	if err != nil {
		t.Fatalf("connConfig: %v", err)
	}
	if connCfg.Host != "db.internal" || connCfg.Port != 5432 {
		t.Errorf("host %q, port %d, want db.internal:5432", connCfg.Host, connCfg.Port)
	}
}

// Finalized is the one check: a config missing any field New reads panics,
// though its ConnTimeout is set.
func TestNew_PanicsOnPartlyFinalizedConfig(t *testing.T) {
	cfg := finalizedConfig(t)
	cfg.MaxOpenConns = nil
	defer func() {
		msg, ok := recover().(string)
		if !ok || !strings.HasPrefix(msg, "postgres:") {
			t.Fatalf("panic = %q, want the provider's finalized-config panic", msg)
		}
	}()
	_, _ = postgres.New(cfg)
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
