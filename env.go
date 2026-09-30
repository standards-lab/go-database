package database

import "github.com/standards-lab/go-core/config"

// Env holds the environment-variable names [Config.Finalize] read.
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

// NewEnv composes the override names under prefix, such as
// PREFIX_DATABASE_HOST.
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
