package exec

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/operation"
)

// Identity is an insert's envelope: the engine-minted identity — a string
// by the organization's UUID convention — and the row's initial version.
type Identity struct {
	ID      string
	Version int64
}

// Insert runs the identity-returning insert inside the consumer's
// transaction, scanning the returned identity and version.
func Insert(ctx context.Context, tx *database.Tx, i operation.Insertion) (Identity, error) {
	stmt, err := i.SQL(tx.Dialect())
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if err := tx.
		QueryRowContext(ctx, stmt.Text, stmt.Args...).
		Scan(&id.ID, &id.Version); err != nil {
		return Identity{}, tx.Dialect().MapError(err)
	}
	return id, nil
}

// Update runs the guarded update, returning the new version on success and
// otherwise the guard outcome: [sql.ErrNoRows] when the row is gone,
// database.ErrVersionMismatch when it moved.
func Update(ctx context.Context, tx *database.Tx, u operation.GuardedUpdate) (int64, error) {
	g, err := u.SQL(tx.Dialect())
	if err != nil {
		return 0, err
	}
	n, err := affected(ctx, tx, g.Command)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		return u.Guard.Version + 1, nil
	}
	return 0, guardFailure(ctx, tx, g.Check, u.Guard.Version)
}

// Delete runs the guarded delete, with the same guard outcomes as [Update].
func Delete(ctx context.Context, tx *database.Tx, del operation.GuardedDelete) error {
	g, err := del.SQL(tx.Dialect())
	if err != nil {
		return err
	}
	n, err := affected(ctx, tx, g.Command)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return guardFailure(ctx, tx, g.Check, del.Guard.Version)
}

// affected runs a command statement and reports its affected-row count.
func affected(ctx context.Context, tx *database.Tx, stmt ast.SQL) (int64, error) {
	res, err := tx.ExecContext(ctx, stmt.Text, stmt.Args...)
	if err != nil {
		return 0, tx.Dialect().MapError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, tx.Dialect().MapError(err)
	}
	return n, nil
}

// guardFailure runs the check statement after a guard miss and classifies
// it: no row is not-found, a row is a version mismatch carrying both
// versions for diagnostics.
func guardFailure(ctx context.Context, tx *database.Tx, check ast.SQL, expected int64) error {
	var current int64
	err := tx.QueryRowContext(ctx, check.Text, check.Args...).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return tx.Dialect().MapError(err)
	}
	return fmt.Errorf("%w: expected %d, current %d", database.ErrVersionMismatch, expected, current)
}
