package postgres_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/postgres"
)

func testDialect(t *testing.T) database.Dialect {
	t.Helper()
	db, err := postgres.New(finalizedConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = db.Conn().Close() })
	return db.Dialect()
}

func TestMapErrorClassifiesConstraints(t *testing.T) {
	d := testDialect(t)

	cases := map[string]struct {
		code  string
		class error
	}{
		"unique":      {"23505", database.ErrUniqueViolation},
		"foreign key": {"23503", database.ErrForeignKeyViolation},
		"check":       {"23514", database.ErrCheckViolation},
		"not null":    {"23502", database.ErrNotNullViolation},
	}
	for name, c := range cases {
		pgErr := &pgconn.PgError{Code: c.code, ConstraintName: "uq_organization_parent_code"}
		err := d.MapError(fmt.Errorf("exec: %w", pgErr))

		if !errors.Is(err, c.class) {
			t.Errorf("%s: errors.Is(err, class) = false; err = %v", name, err)
		}
		var ce *database.ConstraintError
		if !errors.As(err, &ce) || ce.Constraint != "uq_organization_parent_code" {
			t.Errorf("%s: error = %v, want ConstraintError carrying the constraint name", name, err)
			continue
		}
		var reached *pgconn.PgError
		if !errors.As(err, &reached) || reached != pgErr {
			t.Errorf("%s: the driver error is no longer reachable through the wrap", name)
		}
	}
}

func TestMapErrorPassesUnclassifiedThrough(t *testing.T) {
	d := testDialect(t)

	serialization := &pgconn.PgError{Code: "40001"}
	if got := d.MapError(serialization); got != error(serialization) {
		t.Errorf("MapError(40001) = %v, want the error unchanged", got)
	}
	if got := d.MapError(sql.ErrNoRows); got != sql.ErrNoRows {
		t.Errorf("MapError(sql.ErrNoRows) = %v, want it untouched", got)
	}
}

func TestRenderReturning(t *testing.T) {
	d := testDialect(t)

	stmt, err := ast.Insert{
		Table:   "organization",
		Columns: []string{"name"},
		Rows:    [][]any{{"Acme"}},
		Returning: []ast.Column{
			{Expr: ast.Col("id"), Alias: "id"},
			{Expr: ast.Col("version"), Alias: "version"},
		},
	}.Render(d)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if stmt.Text != "INSERT INTO organization (name) VALUES ($1) RETURNING id AS id, version AS version" {
		t.Errorf("sql = %q, want the postgres returning form", stmt.Text)
	}
}
