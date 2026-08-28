package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestCTE(t *testing.T) {
	base := selectOf(col(ast.Col("a")))
	sql, _ := render(t, ast.Select{
		With:    []ast.CTE{{Name: "c", Query: base}},
		Columns: []ast.Column{col(ast.Col("c.a"))},
		From:    ast.Table("c"),
	})
	wantSQL(t, sql, "WITH c AS (SELECT a FROM t) SELECT c.a FROM c")
}

func TestRecursiveCTEWithColumns(t *testing.T) {
	anchor := selectOf(col(ast.Col("a")))
	step := ast.Select{
		Columns: []ast.Column{col(ast.Col("c.a"))},
		From:    ast.Table("c"),
	}
	sql, _ := render(t, ast.Select{
		With: []ast.CTE{{
			Name:      "c",
			Columns:   []string{"a"},
			Recursive: true,
			Query:     ast.UnionAll(anchor, step),
		}},
		Columns: []ast.Column{col(ast.Col("c.a"))},
		From:    ast.Table("c"),
	})
	wantSQL(t, sql, "WITH RECURSIVE c (a) AS (SELECT a FROM t UNION ALL SELECT c.a FROM c) SELECT c.a FROM c")
}

func TestCTEFailures(t *testing.T) {
	base := selectOf(col(ast.Col("a")))
	nested := base
	nested.With = []ast.CTE{{Name: "c", Query: base}}

	cases := map[string]struct {
		s    ast.Select
		want string
	}{
		"missing name": {
			ast.Select{
				With:    []ast.CTE{{Query: base}},
				Columns: []ast.Column{col(ast.Col("a"))},
				From:    ast.Table("c"),
			},
			"missing its name",
		},
		"missing query": {
			ast.Select{
				With:    []ast.CTE{{Name: "c"}},
				Columns: []ast.Column{col(ast.Col("a"))},
				From:    ast.Table("c"),
			},
			"missing its query",
		},
		"WITH in derived table": {
			ast.Select{
				Columns: []ast.Column{col(ast.Col("a"))},
				From:    ast.Derived(nested).As("d"),
			},
			"outermost statement",
		},
	}
	for name, c := range cases {
		err := renderErr(t, c.s)
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestSubqueryOrderRules(t *testing.T) {
	inner := selectOf(col(ast.Col("a")))
	inner.OrderBy = []ast.Order{{Expr: ast.Col("a")}}

	err := renderErr(t, ast.Select{
		Columns: []ast.Column{col(ast.Col("d.a"))},
		From:    ast.Derived(inner).As("d"),
	})
	if !strings.Contains(err.Error(), "orders only when it pages") {
		t.Errorf("error = %v, want the subquery-ordering rule", err)
	}

	paged := inner
	paged.Paging = &ast.Paging{Offset: 5, Fetch: 5}
	sql, args := render(t, ast.Select{
		Columns: []ast.Column{col(ast.Col("d.a"))},
		From:    ast.Derived(paged).As("d"),
	})
	wantSQL(t, sql, "SELECT d.a FROM (SELECT a FROM t ORDER BY a OFFSET $1 ROWS FETCH NEXT $2 ROWS ONLY) d")
	wantArgs(t, args, 5, 5)
}
