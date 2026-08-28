package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Tx is one transaction over the wrapped pool, carrying the dialect so it
// satisfies [Session] and so commit errors can be classified. The consumer
// owns the boundary: [DB.Begin] opens it, [Tx.Commit] or [Tx.Rollback]
// closes it, and [ExecTx] wraps the whole unit.
type Tx struct {
	tx      *sql.Tx
	dialect Dialect
}

// Begin opens a transaction: [ErrNotReady] before Start or after Shutdown,
// and a failure to begin wraps [ErrConnectionFailed].
func (d *DB) Begin(ctx context.Context) (*Tx, error) {
	if !d.started.Load() {
		return nil, ErrNotReady
	}
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	return &Tx{tx: tx, dialect: d.dialect}, nil
}

// Dialect returns the dialect the transaction's database was constructed
// with.
func (t *Tx) Dialect() Dialect {
	return t.dialect
}

// QueryContext runs a query inside the transaction.
func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

// QueryRowContext runs a single-row query inside the transaction.
func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

// ExecContext runs a statement inside the transaction, returning its result.
func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

// Commit commits the transaction, routing any commit error through the
// dialect's MapError — the one place a violation deferred to COMMIT can be
// classified.
func (t *Tx) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return t.dialect.MapError(err)
	}
	return nil
}

// Rollback rolls the transaction back.
func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

// ExecTx runs fn as one unit of work: begin, fn, then commit on success or
// rollback on fn's error, which is returned unchanged. The closure is the
// consumer-owned transaction boundary.
func ExecTx(ctx context.Context, db *DB, fn func(*Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
