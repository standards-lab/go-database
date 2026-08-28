package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestCompoundOperators(t *testing.T) {
	a := selectOf(col(ast.Col("a")))
	b := ast.Select{Columns: []ast.Column{col(ast.Col("b"))}, From: ast.Table("u")}
	cases := map[string]struct {
		c  ast.Compound
		op string
	}{
		"Union":        {ast.Union(a, b), "UNION"},
		"UnionAll":     {ast.UnionAll(a, b), "UNION ALL"},
		"Intersect":    {ast.Intersect(a, b), "INTERSECT"},
		"IntersectAll": {ast.IntersectAll(a, b), "INTERSECT ALL"},
		"Except":       {ast.Except(a, b), "EXCEPT"},
		"ExceptAll":    {ast.ExceptAll(a, b), "EXCEPT ALL"},
	}
	for name, c := range cases {
		sql, _ := render(t, c.c)
		if sql != "SELECT a FROM t "+c.op+" SELECT b FROM u" {
			t.Errorf("%s: sql = %q", name, sql)
		}
	}
}

func TestCompoundTail(t *testing.T) {
	sql, args := render(t, ast.Compound{
		Op:      ast.SetUnion,
		Queries: []ast.Query{selectOf(col(ast.Col("a"))), selectOf(col(ast.Col("a")))},
		OrderBy: []ast.Order{{Expr: ast.Col("a")}},
		Paging:  &ast.Paging{Offset: 10, Fetch: 5},
	})
	wantSQL(t, sql, "SELECT a FROM t UNION SELECT a FROM t ORDER BY a OFFSET $1 ROWS FETCH NEXT $2 ROWS ONLY")
	wantArgs(t, args, 10, 5)
}

func TestCompoundFailures(t *testing.T) {
	a := selectOf(col(ast.Col("a")))
	ordered := a
	ordered.OrderBy = []ast.Order{{Expr: ast.Col("a")}}
	paged := a
	paged.OrderBy = []ast.Order{{Expr: ast.Col("a")}}
	paged.Paging = &ast.Paging{Fetch: 5}
	withCTE := a
	withCTE.With = []ast.CTE{{Name: "c", Query: a}}

	cases := map[string]struct {
		c    ast.Compound
		want string
	}{
		"single query":      {ast.Union(a), "at least two"},
		"non-Select branch": {ast.Union(a, ast.Union(a, a)), "must be Select"},
		"ordered branch":    {ast.Union(a, ordered), "not allowed on a compound branch"},
		"paged branch":      {ast.Union(a, paged), "not allowed on a compound branch"},
		"branch WITH":       {ast.Union(a, withCTE), "outermost statement"},
	}
	for name, c := range cases {
		err := renderErr(t, c.c)
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestCompoundArityPrecedesWith(t *testing.T) {
	a := selectOf(col(ast.Col("a")))
	err := renderErr(t, ast.Compound{
		With:    []ast.CTE{{Query: a}},
		Op:      ast.SetUnion,
		Queries: []ast.Query{a},
	})
	if !strings.Contains(err.Error(), "at least two") {
		t.Errorf("error = %v, want the compound's own defect before the CTE's", err)
	}
}
