package database

import (
	"context"
	"database/sql"
)

// Session is the querying surface higher layers run statements against,
// implemented by both [DB] and [Tx]: the dialect travels with the session,
// and the same read path works against the pool or inside a transaction.
// Write runners deliberately take *Tx, not Session, so the compiler enforces
// the consumer-owned transaction boundary.
type Session interface {
	Dialect() Dialect
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// QueryContext runs a query against the pool.

func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.conn.QueryContext(ctx, query, args...)
}

// QueryRowContext runs a single-row query against the pool.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.conn.QueryRowContext(ctx, query, args...)
}

// ExecContext runs a statement against the pool, returning its result.

func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.conn.ExecContext(ctx, query, args...)
}
