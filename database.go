package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"
)

// DB is a provider-constructed connection pool with lifecycle hooks.
type DB struct {
	conn        *sql.DB
	connTimeout time.Duration
	started     atomic.Bool
}

// New wraps a provider-constructed pool and applies cfg's pool settings. It
// panics on an unfinalized cfg or a nil conn.
func New(conn *sql.DB, cfg Config) *DB {
	if !cfg.Finalized() {
		panic("database: Config not finalized: call Finalize before New")
	}
	if conn == nil {
		panic("database: nil conn")
	}
	conn.SetMaxOpenConns(*cfg.MaxOpenConns)
	conn.SetMaxIdleConns(*cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(cfg.ConnMaxLifetime.Duration())
	conn.SetConnMaxIdleTime(cfg.ConnMaxIdleTime.Duration())

	return &DB{
		conn:        conn,
		connTimeout: cfg.ConnTimeout.Duration(),
	}
}

// Conn returns the underlying connection pool.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// ConnTimeout returns the configured conn_timeout, the bound Start and Ready
// apply to a ping.
func (d *DB) ConnTimeout() time.Duration {
	return d.connTimeout
}

// Start pings the database, bounded by conn_timeout, and marks it started; a
// failure wraps [ErrConnectionFailed] around the driver's error.
func (d *DB) Start(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, d.connTimeout)
	defer cancel()
	if err := d.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	d.started.Store(true)
	return nil
}

// Shutdown clears readiness and closes the pool, returning the close error;
// closing a pool that never connected is a no-op.
func (d *DB) Shutdown(ctx context.Context) error {
	d.started.Store(false)
	return d.conn.Close()
}

// Ready reports whether Start succeeded and a ping bounded by conn_timeout
// succeeds now.
func (d *DB) Ready() bool {
	if !d.started.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), d.connTimeout)
	defer cancel()
	return d.conn.PingContext(ctx) == nil
}

// Ping pings the database under the caller's context: [ErrNotReady] before
// Start or after Shutdown, and a failure wraps [ErrConnectionFailed].
func (d *DB) Ping(ctx context.Context) error {
	if !d.started.Load() {
		return ErrNotReady
	}
	if err := d.conn.PingContext(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	return nil
}
