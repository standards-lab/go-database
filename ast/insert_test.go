package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestInsertValues(t *testing.T) {
	sql, args := render(t, ast.Insert{
		Table:   "organization",
		Columns: []string{"parent_id", "code", "name"},
		Rows:    [][]any{{nil, "acme", "Acme"}},
	})
	wantSQL(t, sql, "INSERT INTO organization (parent_id, code, name) VALUES ($1, $2, $3)")
	wantArgs(t, args, nil, "acme", "Acme")
}

func TestInsertMultipleRows(t *testing.T) {
	sql, args := render(t, ast.Insert{
		Table:   "t",
		Columns: []string{"a"},
		Rows:    [][]any{{1}, {2}},
	})
	wantSQL(t, sql, "INSERT INTO t (a) VALUES ($1), ($2)")
	wantArgs(t, args, 1, 2)
}

func TestInsertExpressionValue(t *testing.T) {
	sql, args := render(t, ast.Insert{
		Table:   "t",
		Columns: []string{"id", "name"},
		Rows:    [][]any{{ast.Raw("uuidv7()"), "Acme"}},
	})
	wantSQL(t, sql, "INSERT INTO t (id, name) VALUES (uuidv7(), $1)")
	wantArgs(t, args, "Acme")
}

func TestInsertSelect(t *testing.T) {
	sql, _ := render(t, ast.Insert{
		Table:   "archive",
		Columns: []string{"a"},
		Query:   selectOf(col(ast.Col("a"))),
	})
	wantSQL(t, sql, "INSERT INTO archive (a) SELECT a FROM t")
}

func TestInsertWithoutColumnList(t *testing.T) {
	sql, args := render(t, ast.Insert{
		Table: "t",
		Rows:  [][]any{{1, 2}},
	})
	wantSQL(t, sql, "INSERT INTO t VALUES ($1, $2)")
	wantArgs(t, args, 1, 2)
}

func TestInsertFailures(t *testing.T) {
	cases := map[string]struct {
		i    ast.Insert
		want string
	}{
		"missing table": {
			ast.Insert{Rows: [][]any{{1}}},
			"missing its table",
		},
		"rows and query": {
			ast.Insert{Table: "t", Rows: [][]any{{1}}, Query: selectOf(col(ast.Col("a")))},
			"not both",
		},
		"neither rows nor query": {
			ast.Insert{Table: "t"},
			"missing rows or a query",
		},
		"empty column": {
			ast.Insert{Table: "t", Columns: []string{""}, Rows: [][]any{{1}}},
			"empty insert column",
		},
		"empty row": {
			ast.Insert{Table: "t", Rows: [][]any{{}}},
			"empty insert row",
		},
		"row width mismatch": {
			ast.Insert{Table: "t", Columns: []string{"a", "b"}, Rows: [][]any{{1}}},
			"row width",
		},
	}
	for name, c := range cases {
		err := renderErr(t, c.i)
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestInsertReturning(t *testing.T) {
	i := ast.Insert{
		Table:   "t",
		Columns: []string{"name"},
		Rows:    [][]any{{"Acme"}},
		Returning: []ast.Column{
			{Expr: ast.Col("id"), Alias: "id"},
			{Expr: ast.Col("version"), Alias: "version"},
		},
	}

	res, err := i.Render(returningStub{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	wantSQL(t, res.Text, "INSERT INTO t (name) VALUES ($1) RETURNING id AS id, version AS version")
	wantArgs(t, res.Args, "Acme")
}

func TestReturningWithoutCapabilityFailsTyped(t *testing.T) {
	i := ast.Insert{
		Table:     "t",
		Columns:   []string{"name"},
		Rows:      [][]any{{"Acme"}},
		Returning: []ast.Column{{Expr: ast.Col("id")}},
	}

	_, err := i.Render(stub{})
	var unsupported *ast.UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v, want UnsupportedFeatureError", err)
	}
	if unsupported.Feature != ast.FeatureReturning || unsupported.Dialect != "stub" {
		t.Errorf("error = %+v, want Feature %q on dialect %q", unsupported, ast.FeatureReturning, "stub")
	}
}
