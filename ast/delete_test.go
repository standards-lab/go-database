package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestDelete(t *testing.T) {
	sql, args := render(t, ast.Delete{
		Table: "organization",
		Where: ast.And(
			ast.Col("id").Eq("x"),
			ast.Col("version").Eq(3),
		),
	})
	wantSQL(t, sql, "DELETE FROM organization WHERE (id = $1 AND version = $2)")
	wantArgs(t, args, "x", 3)
}

func TestDeleteWithoutWhere(t *testing.T) {
	sql, _ := render(t, ast.Delete{Table: "t"})
	wantSQL(t, sql, "DELETE FROM t")
}

func TestDeleteMissingTableFails(t *testing.T) {
	err := renderErr(t, ast.Delete{})
	if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), "missing its table") {
		t.Errorf("error = %v, want ErrInvalidStatement with missing its table", err)
	}
}

func TestDeleteReturning(t *testing.T) {
	d := ast.Delete{
		Table:     "t",
		Where:     ast.Col("id").Eq("x"),
		Returning: []ast.Column{{Expr: ast.Col("id")}},
	}

	res, err := d.Render(returningStub{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	wantSQL(t, res.Text, "DELETE FROM t WHERE id = $1 RETURNING id")

	if _, err := d.Render(stub{}); err == nil {
		t.Error("Render() on a dialect without the capability succeeded, want typed error")
	}
}
