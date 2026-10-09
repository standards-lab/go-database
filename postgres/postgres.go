package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/standards-lab/go-database"
)

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
// with [database.New], without I/O. An unfinalized config panics; a
// reserved option or a config pgx cannot parse is an error.
func New(cfg database.Config) (*database.DB, error) {
	if !cfg.Finalized() {
		panic("postgres: Config not finalized: call Finalize before New")
	}
	connCfg, err := connConfig(cfg)
	if err != nil {
		return nil, err
	}
	return database.New(stdlib.OpenDB(*connCfg, stdlib.OptionAfterConnect(scanUTC)), cfg), nil
}

// scanUTC registers, on each new connection's type map, a timestamptz codec
// that returns its values in time.UTC; pgx's default codec returns them in
// time.Local, and the session's TimeZone does not change that. timestamp
// needs no codec: pgx already returns it in time.UTC.
func scanUTC(_ context.Context, conn *pgx.Conn) error {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})
	return nil
}

// connConfig composes the connection URL from cfg, parses it, and sets the
// password and connect timeout on the parsed config.
func connConfig(cfg database.Config) (*pgx.ConnConfig, error) {
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
		Scheme: "postgres",
		Path:   "/" + cfg.Name,
	}
	if strings.HasPrefix(cfg.Host, "/") {
		// A Unix-socket directory is no URL authority: pgx reads it, and
		// the port naming the socket file, from the query.
		query.Set("host", cfg.Host)
		query.Set("port", strconv.Itoa(port))
	} else {
		u.Host = net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	}
	u.RawQuery = query.Encode()

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
	return connCfg, nil
}
