package postgres

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/standards-lab/go-database"
)

// Provider is this sub-module's typed selection constant.
const Provider database.Provider = "postgres"

const defaultPort = 5432

var reservedOptions = map[string]bool{
	"host":            true,
	"port":            true,
	"user":            true,
	"password":        true,
	"dbname":          true,
	"database":        true,
	"connect_timeout": true,
}

// New constructs the connection pool from a finalized config and wraps it
// with the postgres dialect via [database.New]. It performs no I/O; see the
// package documentation for the composition and its guarantees. An
// unfinalized config panics with the fix named.
func New(cfg database.Config) (*database.DB, error) {
	if cfg.ConnTimeout == nil {
		panic("postgres: Config not finalized: call Finalize before New")
	}
	for key := range cfg.Options {
		if reservedOptions[key] {
			return nil, fmt.Errorf("option %q conflicts with a connection field", key)
		}
	}

	port := defaultPort
	if cfg.Port != nil {
		port = *cfg.Port
	}

	query := url.Values{}
	for k, v := range cfg.Options {
		query.Set(k, v)
	}

	u := url.URL{
		Scheme:   string(Provider),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(port)),
		Path:     "/" + cfg.Name,
		RawQuery: query.Encode(),
	}

	if cfg.User != "" {
		u.User = url.User(cfg.User)
	}

	connCfg, err := pgx.ParseConfig(u.String())
	if err != nil {
		return nil, fmt.Errorf("parse connection config: %w", err)
	}
	if cfg.Password != "" {
		connCfg.Password = cfg.Password
	}
	connCfg.ConnectTimeout = cfg.ConnTimeout.Duration()

	return database.New(
		stdlib.OpenDB(*connCfg), dialect{}, cfg,
	), nil
}
