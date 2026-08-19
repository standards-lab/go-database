package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"
)

// DB wraps a provider-constructed connection pool with lifecycle integration
// and the provider's dialect. It owns one pool for the process: construction
// performs no I/O, Start establishes connectivity, and Ready reports it live.
// Start and Shutdown carry the lifecycle hook signature, so the composition
// root registers the bare method values, and Ready satisfies
// lifecycle.ReadinessChecker structurally; the package registers no hooks of
// its own.
type DB struct {
	conn        *sql.DB
	dialect     Dialect
	connTimeout time.Duration
	started     atomic.Bool
}

// New wraps a provider-constructed pool with dialect and applies cfg's pool
// settings — the dialect-independent half of construction every provider
// shares. It panics if cfg was not finalized, or on a nil conn or dialect:
// each is a wiring defect at the composition root, not a runtime condition.
func New(conn *sql.DB, dialect Dialect, cfg Config) *DB {
	if !cfg.finalized() {
		panic("database: Config not finalized: call Finalize before New")
	}
	if conn == nil {
		panic("database: nil conn")
	}
	if dialect == nil {
		panic("database: nil dialect")
	}
	conn.SetMaxOpenConns(*cfg.MaxOpenConns)
	conn.SetMaxIdleConns(*cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(cfg.ConnMaxLifetime.Duration())
	conn.SetConnMaxIdleTime(cfg.ConnMaxIdleTime.Duration())

	return &DB{
		conn:        conn,
		dialect:     dialect,
		connTimeout: cfg.ConnTimeout.Duration(),
	}
}

// Conn returns the underlying connection pool.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// Dialect returns the dialect the provider constructed this database with.
func (d *DB) Dialect() Dialect {
	return d.dialect
}

// Start establishes connectivity with a ping bounded by the configured
// conn_timeout and marks the database started. A failure wraps
// [ErrConnectionFailed] around the driver's error. There is no started guard:
// a repeat Start just pings again.
func (d *DB) Start(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, d.connTimeout)
	defer cancel()
	if err := d.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	d.started.Store(true)
	return nil
}

// Shutdown clears readiness and closes the pool, returning the close error.
// Closing a pool that never connected is a clean no-op, so the
// drain-after-failed-startup path is safe. The context exists for the
// lifecycle hook signature; sql.DB.Close is synchronous.
func (d *DB) Shutdown(ctx context.Context) error {
	d.started.Store(false)
	return d.conn.Close()
}

// Ready reports live connectivity: false before Start or after Shutdown, and
// otherwise the result of a ping bounded by the configured conn_timeout. The
// probe reflects the database now — readiness drops during an outage and
// recovers with it, at the cost of one bounded round trip per call.
func (d *DB) Ready() bool {
	if !d.started.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), d.connTimeout)
	defer cancel()
	return d.conn.PingContext(ctx) == nil
}

// Ping verifies connectivity under the caller's context: [ErrNotReady] before
// Start or after Shutdown, and otherwise a ping whose failure wraps
// [ErrConnectionFailed]. Unlike [DB.Ready] it applies no timeout of its own.
func (d *DB) Ping(ctx context.Context) error {
	if !d.started.Load() {
		return ErrNotReady
	}
	if err := d.conn.PingContext(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	return nil
}
