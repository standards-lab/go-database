package database_test

import (
	"strings"
	"testing"
	"time"

	"github.com/standards-lab/go-core/config"
	"github.com/standards-lab/go-database"
)

// validConfig satisfies the one required field; everything else exercises
// defaults under test.
func validConfig() database.Config {
	return database.Config{Name: "app"}
}

func TestConfig_MergeOverlaysSetFields(t *testing.T) {
	base := database.Config{
		Host: "localhost",
		Name: "app",
		User: "app",
		Port: new(5432),
	}
	overlay := database.Config{
		Host: "db.internal",
		Port: new(5433),
	}

	base.Merge(&overlay)

	if base.Host != "db.internal" {
		t.Errorf("Host = %s, want db.internal", base.Host)
	}
	if base.Port == nil || *base.Port != 5433 {
		t.Errorf("Port = %v, want 5433", base.Port)
	}
	// Fields the overlay leaves unset keep the base values.
	if base.Name != "app" {
		t.Errorf("Name = %s, want app", base.Name)
	}
	if base.User != "app" {
		t.Errorf("User = %s, want app", base.User)
	}
}

func TestConfig_MergeOptionsKeyWise(t *testing.T) {
	base := database.Config{
		Name:    "app",
		Options: map[string]string{"sslmode": "disable", "application_name": "svc"},
	}
	overlay := database.Config{
		Options: map[string]string{"sslmode": "require"},
	}

	base.Merge(&overlay)

	if got := base.Options["sslmode"]; got != "require" {
		t.Errorf("Options[sslmode] = %s, want require", got)
	}
	// An overlay key overrides without dropping the others.
	if got := base.Options["application_name"]; got != "svc" {
		t.Errorf("Options[application_name] = %s, want svc", got)
	}
}

func TestConfig_MergeOptionsOntoNilMap(t *testing.T) {
	base := database.Config{Name: "app"}
	overlay := database.Config{Options: map[string]string{"sslmode": "disable"}}

	base.Merge(&overlay)

	if got := base.Options["sslmode"]; got != "disable" {
		t.Errorf("Options[sslmode] = %s, want disable", got)
	}
}

func TestConfig_FinalizeDefaults(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host = %s, want localhost", cfg.Host)
	}
	// Port keeps no base default: 5432 is a provider fact.
	if cfg.Port != nil {
		t.Errorf("Port = %v, want nil", cfg.Port)
	}
	if cfg.MaxOpenConns == nil || *cfg.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %v, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns == nil || *cfg.MaxIdleConns != 5 {
		t.Errorf("MaxIdleConns = %v, want 5", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime == nil || cfg.ConnMaxLifetime.Duration() != 15*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 15m", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime == nil || cfg.ConnMaxIdleTime.Duration() != 5*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 5m", cfg.ConnMaxIdleTime)
	}
	if cfg.ConnTimeout == nil || cfg.ConnTimeout.Duration() != 5*time.Second {
		t.Errorf("ConnTimeout = %v, want 5s", cfg.ConnTimeout)
	}
}

func TestConfig_FinalizeRequiresName(t *testing.T) {
	cfg := database.Config{User: "app"}
	err := cfg.Finalize("")
	if err == nil {
		t.Fatal("Finalize accepted a config with no database name")
	}
	if !strings.Contains(err.Error(), "name required") {
		t.Errorf("error = %v, want it to name the missing field", err)
	}
}

func TestConfig_FinalizeUserOptional(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Finalize(""); err != nil {
		t.Errorf("Finalize rejected an empty user: %v", err)
	}
}

func TestConfig_FinalizeEnvOverrides(t *testing.T) {
	cfg := validConfig()

	t.Setenv("TEST_DATABASE_HOST", "db.internal")
	t.Setenv("TEST_DATABASE_PORT", "5433")
	t.Setenv("TEST_DATABASE_NAME", "app_test")
	t.Setenv("TEST_DATABASE_USER", "tester")
	t.Setenv("TEST_DATABASE_PASSWORD", "secret")
	t.Setenv("TEST_DATABASE_MAX_OPEN_CONNS", "50")
	t.Setenv("TEST_DATABASE_MAX_IDLE_CONNS", "10")
	t.Setenv("TEST_DATABASE_CONN_MAX_LIFETIME", "30m")
	t.Setenv("TEST_DATABASE_CONN_MAX_IDLE_TIME", "10m")
	t.Setenv("TEST_DATABASE_CONN_TIMEOUT", "3s")

	if err := cfg.Finalize("test"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	if cfg.Host != "db.internal" {
		t.Errorf("Host = %s, want db.internal", cfg.Host)
	}
	if cfg.Port == nil || *cfg.Port != 5433 {
		t.Errorf("Port = %v, want 5433", cfg.Port)
	}
	if cfg.Name != "app_test" {
		t.Errorf("Name = %s, want app_test", cfg.Name)
	}
	if cfg.User != "tester" {
		t.Errorf("User = %s, want tester", cfg.User)
	}
	if cfg.Password != "secret" {
		t.Errorf("Password = %s, want secret", cfg.Password)
	}
	if cfg.MaxOpenConns == nil || *cfg.MaxOpenConns != 50 {
		t.Errorf("MaxOpenConns = %v, want 50", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns == nil || *cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %v, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime.Duration() != 30*time.Minute {
		t.Errorf("ConnMaxLifetime = %s, want 30m", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime.Duration() != 10*time.Minute {
		t.Errorf("ConnMaxIdleTime = %s, want 10m", cfg.ConnMaxIdleTime)
	}
	if cfg.ConnTimeout.Duration() != 3*time.Second {
		t.Errorf("ConnTimeout = %s, want 3s", cfg.ConnTimeout)
	}
}

func TestConfig_FinalizeMalformedEnvFails(t *testing.T) {
	cases := []struct {
		name  string
		set   string
		value string
	}{
		{"port", "TEST_DATABASE_PORT", "not-a-number"},
		{"max open conns", "TEST_DATABASE_MAX_OPEN_CONNS", "many"},
		{"max idle conns", "TEST_DATABASE_MAX_IDLE_CONNS", "few"},
		{"conn timeout", "TEST_DATABASE_CONN_TIMEOUT", "soon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			t.Setenv(tc.set, tc.value)

			err := cfg.Finalize("test")
			if err == nil {
				t.Fatalf("Finalize accepted %s=%q", tc.set, tc.value)
			}
			if !strings.Contains(err.Error(), tc.set) {
				t.Errorf("error = %v, want it to name %s", err, tc.set)
			}
		})
	}
}

func TestConfig_FinalizeZeroEnvDisablesOverrides(t *testing.T) {
	// The zero Env names no variables, so ambient values cannot leak in.
	t.Setenv("TEST_DATABASE_HOST", "db.internal")

	cfg := validConfig()
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %s with zero Env, want localhost", cfg.Host)
	}
}

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*database.Config)
		wantErr string
	}{
		{
			"port zero",
			func(c *database.Config) { c.Port = new(0) },
			"invalid port",
		},
		{
			"port out of range",
			func(c *database.Config) { c.Port = new(70000) },
			"invalid port",
		},
		{
			"negative max open conns",
			func(c *database.Config) { c.MaxOpenConns = new(-1) },
			"invalid max_open_conns",
		},
		{
			"negative max idle conns",
			func(c *database.Config) { c.MaxIdleConns = new(-1) },
			"invalid max_idle_conns",
		},
		{
			"idle exceeds open",
			func(c *database.Config) {
				c.MaxOpenConns = new(5)
				c.MaxIdleConns = new(10)
			},
			"must not exceed",
		},
		{
			"negative conn max lifetime",
			func(c *database.Config) {
				c.ConnMaxLifetime = new(config.Duration(-time.Minute))
			},
			"invalid conn_max_lifetime",
		},
		{
			"negative conn max idle time",
			func(c *database.Config) {
				c.ConnMaxIdleTime = new(config.Duration(-time.Minute))
			},
			"invalid conn_max_idle_time",
		},
		{
			"zero conn timeout",
			func(c *database.Config) {
				c.ConnTimeout = new(config.Duration(0))
			},
			"conn_timeout must be positive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			err := cfg.Finalize("")
			if err == nil {
				t.Fatal("Finalize accepted an invalid config")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestConfig_ValidateAllowsZeroPools(t *testing.T) {
	// Zero means unlimited open connections and no idle pool, per
	// database/sql semantics; both survive validation.
	cfg := validConfig()
	cfg.MaxOpenConns = new(0)
	cfg.MaxIdleConns = new(0)

	if err := cfg.Finalize(""); err != nil {
		t.Errorf("Finalize rejected zero pool settings: %v", err)
	}
}
