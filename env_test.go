package database_test

import (
	"testing"

	"github.com/standards-lab/go-database"
)

// Every name derives from the prefix through config.EnvName with the
// "database" segment; this pins the full set a consumer binds to.
func TestNewEnv(t *testing.T) {
	env := database.NewEnv("app")

	cases := []struct {
		field string
		got   string
		want  string
	}{
		{"Host", env.Host, "APP_DATABASE_HOST"},
		{"Port", env.Port, "APP_DATABASE_PORT"},
		{"Name", env.Name, "APP_DATABASE_NAME"},
		{"User", env.User, "APP_DATABASE_USER"},
		{"Password", env.Password, "APP_DATABASE_PASSWORD"},
		{"MaxOpenConns", env.MaxOpenConns, "APP_DATABASE_MAX_OPEN_CONNS"},
		{"MaxIdleConns", env.MaxIdleConns, "APP_DATABASE_MAX_IDLE_CONNS"},
		{"ConnMaxLifetime", env.ConnMaxLifetime, "APP_DATABASE_CONN_MAX_LIFETIME"},
		{"ConnMaxIdleTime", env.ConnMaxIdleTime, "APP_DATABASE_CONN_MAX_IDLE_TIME"},
		{"ConnTimeout", env.ConnTimeout, "APP_DATABASE_CONN_TIMEOUT"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %s, want %s", tc.field, tc.got, tc.want)
		}
	}
}

func TestNewEnv_EmptyPrefixReturnsZeroEnv(t *testing.T) {
	if env := database.NewEnv(""); env != (database.Env{}) {
		t.Errorf("NewEnv(\"\") = %+v, want the zero Env (overrides disabled)", env)
	}
}
