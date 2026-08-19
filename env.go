package database

import "github.com/standards-lab/go-core/config"

// Env names the environment variables [Config.Finalize] reads, composed from
// the prefix it receives under the "database" segment: DATABASE_HOST,
// DATABASE_PORT, and the rest, prefixed by whatever [config.EnvName]
// produces. An empty name disables that one override; the zero value — an
// empty prefix — disables all of them. Populated by Finalize and exposed for
// introspection.
type Env struct {
	Host            string
	Name            string
	User            string
	Password        string
	Port            string
	MaxOpenConns    string
	MaxIdleConns    string
	ConnMaxLifetime string
	ConnMaxIdleTime string
	ConnTimeout     string
}

// NewEnv composes the standard override names from a prefix under the
// "database" segment: DATABASE_HOST, DATABASE_PORT, and the rest, prefixed by
// whatever [config.EnvName] produces. An empty prefix returns the zero Env,
// disabling the overrides.
func NewEnv(prefix string) Env {
	if prefix == "" {
		return Env{}
	}
	return Env{
		Host: config.EnvName(
			prefix, "database", "host",
		),
		Name: config.EnvName(
			prefix, "database", "name",
		),
		User: config.EnvName(
			prefix, "database", "user",
		),
		Password: config.EnvName(
			prefix, "database", "password",
		),
		Port: config.EnvName(
			prefix, "database", "port",
		),
		MaxOpenConns: config.EnvName(
			prefix, "database", "max", "open", "conns",
		),
		MaxIdleConns: config.EnvName(
			prefix, "database", "max", "idle", "conns",
		),
		ConnMaxLifetime: config.EnvName(
			prefix, "database", "conn", "max", "lifetime",
		),
		ConnMaxIdleTime: config.EnvName(
			prefix, "database", "conn", "max", "idle", "time",
		),
		ConnTimeout: config.EnvName(
			prefix, "database", "conn", "timeout",
		),
	}
}
