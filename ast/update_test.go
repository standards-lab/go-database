package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestUpdate(t *testing.T) {
	sql, args := render(t, ast.Update{
		Table: "organization",
		Set: []ast.Assignment{
			{Column: "name", Value: "Acme"},
			{Column: "version", Value: ast.Raw("version + 1")},
		},
		Where: ast.And(
			ast.Col("id").Eq("x"),
			ast.Col("version").Eq(3),
		),
	})
	wantSQL(t, sql, "UPDATE organization SET name = $1, version = version + 1 WHERE (id = $2 AND version = $3)")
	wantArgs(t, args, "Acme", "x", 3)
}

func TestUpdateWithoutWhere(t *testing.T) {
	sql, args := render(t, ast.Update{
		Table: "t",
		Set:   []ast.Assignment{{Column: "a", Value: 1}},
	})
	wantSQL(t, sql, "UPDATE t SET a = $1")
	wantArgs(t, args, 1)
}

func TestUpdateFailures(t *testing.T) {
	cases := map[string]struct {
		u    ast.Update
		want string
	}{
		"missing table": {
			ast.Update{Set: []ast.Assignment{{Column: "a", Value: 1}}},
			"missing its table",
		},
		"missing SET": {
			ast.Update{Table: "t"},
			"missing its SET list",
		},
		"assignment without column": {
			ast.Update{Table: "t", Set: []ast.Assignment{{Value: 1}}},
			"missing its column",
		},
	}
	for name, c := range cases {
		err := renderErr(t, c.u)
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestUpdateReturning(t *testing.T) {
	u := ast.Update{
		Table:     "t",
		Set:       []ast.Assignment{{Column: "a", Value: 1}},
		Returning: []ast.Column{{Expr: ast.Col("version")}},
	}

	res, err := u.Render(returningStub{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	wantSQL(t, res.Text, "UPDATE t SET a = $1 RETURNING version")

	if _, err := u.Render(stub{}); err == nil {
		t.Error("Render() on a dialect without the capability succeeded, want typed error")
	}
}
