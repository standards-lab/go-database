package ast_test

import (
	"errors"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestColSpellingsAreEquivalent(t *testing.T) {
	dotted, _ := render(t, selectOf(col(ast.Col("o.name"))))
	parts, _ := render(t, selectOf(col(ast.Col("o", "name"))))
	wantSQL(t, dotted, "SELECT o.name FROM t")
	if parts != dotted {
		t.Errorf("Col(\"o\", \"name\") = %q, Col(\"o.name\") = %q; want equal", parts, dotted)
	}
}

func TestColFailures(t *testing.T) {
	for name, e := range map[string]ast.Expression{
		"no parts":   ast.Col(),
		"empty part": ast.Col("o", ""),
	} {
		if err := renderErr(t, selectOf(col(e))); !errors.Is(err, ast.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestRaw(t *testing.T) {
	sql, args := render(t, selectOf(col(ast.Raw("*"))))
	wantSQL(t, sql, "SELECT * FROM t")
	wantArgs(t, args)

	if err := renderErr(t, selectOf(col(ast.Raw("")))); !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("empty Raw: error = %v, want ErrInvalidStatement", err)
	}
}

func TestValBinds(t *testing.T) {
	sql, args := render(t, selectOf(ast.Val(42).As("answer")))
	wantSQL(t, sql, "SELECT $1 AS answer FROM t")
	wantArgs(t, args, 42)
}

func TestFnLiftsArguments(t *testing.T) {
	sql, args := render(t, selectOf(col(ast.Fn("COALESCE", ast.Col("nickname"), "unknown"))))
	wantSQL(t, sql, "SELECT COALESCE(nickname, $1) FROM t")
	wantArgs(t, args, "unknown")
}

func TestFnEmptyNameFails(t *testing.T) {
	if err := renderErr(t, selectOf(col(ast.Fn("")))); !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestSelectAsExpression(t *testing.T) {
	max := selectOf(col(ast.Fn("MAX", ast.Col("age"))))
	sql, args := render(t, ast.Select{
		Columns: []ast.Column{col(ast.Col("name"))},
		From:    ast.Table("t"),
		Where:   ast.Col("age").Eq(max),
	})
	wantSQL(t, sql, "SELECT name FROM t WHERE age = (SELECT MAX(age) FROM t)")
	wantArgs(t, args)
}

func TestZeroExpressionFails(t *testing.T) {
	if err := renderErr(t, selectOf(ast.Column{})); !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}
