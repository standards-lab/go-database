package postgres

import (
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
)

// dialect is the postgres seam: identity, $N placeholders, constraint
// classification, and the returning capability.
type dialect struct{}

func (dialect) Name() string {
	return string(Provider)
}

func (dialect) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

// MapError classifies SQLSTATE class-23 constraint violations into the base
// package's sentinels through a ConstraintError carrying the constraint
// name pgx exposes structurally. Everything else — sql.ErrNoRows included —
// passes through unchanged, the driver error always reachable.
func (dialect) MapError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	var class error
	switch pgErr.Code {
	case "23505":
		class = database.ErrUniqueViolation
	case "23503":
		class = database.ErrForeignKeyViolation
	case "23514":
		class = database.ErrCheckViolation
	case "23502":
		class = database.ErrNotNullViolation
	default:
		return err
	}
	return &database.ConstraintError{
		Constraint: pgErr.ConstraintName,
		Class:      class,
		Err:        err,
	}
}

// RenderReturning opts the provider into the declared-native returning
// clause, emitting the postgres RETURNING form.
func (dialect) RenderReturning(w ast.Writer, cols []ast.Column) {
	w.SQL("RETURNING ")
	for i, c := range cols {
		if i > 0 {
			w.SQL(", ")
		}
		w.Column(c)
	}
}
