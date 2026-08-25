package query_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/query"
)

func TestSelectAllClauses(t *testing.T) {
	sql, args := render(t, query.Select{
		Distinct: true,
		Columns: []query.Column{
			col(query.Col("kind")),
			query.Fn("COUNT", query.Raw("*")).As("total"),
		},
		From:    query.Table("t"),
		Where:   query.Col("age").Ge(21),
		GroupBy: []query.Expression{query.Col("kind")},
		Having:  query.Fn("COUNT", query.Raw("*")).Gt(1),
		OrderBy: []query.Order{{Expr: query.Col("kind")}},
		Paging:  &query.Paging{Offset: 0, Fetch: 5},
	})
	wantSQL(t, sql,
		"SELECT DISTINCT kind, COUNT(*) AS total FROM t WHERE age >= $1 "+
			"GROUP BY kind HAVING COUNT(*) > $2 ORDER BY kind "+
			"OFFSET $3 ROWS FETCH NEXT $4 ROWS ONLY")
	wantArgs(t, args, 21, 1, 0, 5)
}

func TestEmptySelectListFails(t *testing.T) {
	err := renderErr(t, query.Select{From: query.Table("t")})
	if !errors.Is(err, query.ErrInvalidStatement) || !strings.Contains(err.Error(), "empty select list") {
		t.Errorf("error = %v, want ErrInvalidStatement with empty select list", err)
	}
}

func TestFirstFailureWins(t *testing.T) {
	err := renderErr(t, query.Select{})
	if !strings.Contains(err.Error(), "empty select list") {
		t.Errorf("error = %v, want the first defect (empty select list)", err)
	}
}

func TestOrderByDescending(t *testing.T) {
	sql, _ := render(t, query.Select{
		Columns: []query.Column{col(query.Col("a"))},
		From:    query.Table("t"),
		OrderBy: []query.Order{{Expr: query.Col("a")}, {Expr: query.Col("b"), Descending: true}},
	})
	wantSQL(t, sql, "SELECT a FROM t ORDER BY a, b DESC")
}

func TestCompoundOperators(t *testing.T) {
	a := selectOf(col(query.Col("a")))
	b := query.Select{Columns: []query.Column{col(query.Col("b"))}, From: query.Table("u")}
	cases := map[string]struct {
		c  query.Compound
		op string
	}{
		"Union":        {query.Union(a, b), "UNION"},
		"UnionAll":     {query.UnionAll(a, b), "UNION ALL"},
		"Intersect":    {query.Intersect(a, b), "INTERSECT"},
		"IntersectAll": {query.IntersectAll(a, b), "INTERSECT ALL"},
		"Except":       {query.Except(a, b), "EXCEPT"},
		"ExceptAll":    {query.ExceptAll(a, b), "EXCEPT ALL"},
	}
	for name, c := range cases {
		sql, _ := render(t, c.c)
		wantSQL(t, sql, "SELECT a FROM t "+c.op+" SELECT b FROM u")
		_ = name
	}
}

func TestCompoundTail(t *testing.T) {
	sql, args := render(t, query.Compound{
		Op:      query.SetUnion,
		Queries: []query.Query{selectOf(col(query.Col("a"))), selectOf(col(query.Col("a")))},
		OrderBy: []query.Order{{Expr: query.Col("a")}},
		Paging:  &query.Paging{Offset: 10, Fetch: 5},
	})
	wantSQL(t, sql, "SELECT a FROM t UNION SELECT a FROM t ORDER BY a OFFSET $1 ROWS FETCH NEXT $2 ROWS ONLY")
	wantArgs(t, args, 10, 5)
}

func TestCompoundFailures(t *testing.T) {
	a := selectOf(col(query.Col("a")))
	ordered := a
	ordered.OrderBy = []query.Order{{Expr: query.Col("a")}}
	paged := a
	paged.OrderBy = []query.Order{{Expr: query.Col("a")}}
	paged.Paging = &query.Paging{Fetch: 5}
	withCTE := a
	withCTE.With = []query.CTE{{Name: "c", Query: a}}

	cases := map[string]struct {
		c    query.Compound
		want string
	}{
		"single query":      {query.Union(a), "at least two"},
		"non-Select branch": {query.Union(a, query.Union(a, a)), "must be Select"},
		"ordered branch":    {query.Union(a, ordered), "not allowed on a compound branch"},
		"paged branch":      {query.Union(a, paged), "not allowed on a compound branch"},
		"branch WITH":       {query.Union(a, withCTE), "outermost statement"},
	}
	for name, c := range cases {
		err := renderErr(t, c.c)
		if !errors.Is(err, query.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestCTE(t *testing.T) {
	base := selectOf(col(query.Col("a")))
	sql, _ := render(t, query.Select{
		With:    []query.CTE{{Name: "c", Query: base}},
		Columns: []query.Column{col(query.Col("c.a"))},
		From:    query.Table("c"),
	})
	wantSQL(t, sql, "WITH c AS (SELECT a FROM t) SELECT c.a FROM c")
}

func TestRecursiveCTEWithColumns(t *testing.T) {
	anchor := selectOf(col(query.Col("a")))
	step := query.Select{
		Columns: []query.Column{col(query.Col("c.a"))},
		From:    query.Table("c"),
	}
	sql, _ := render(t, query.Select{
		With: []query.CTE{{
			Name:      "c",
			Columns:   []string{"a"},
			Recursive: true,
			Query:     query.UnionAll(anchor, step),
		}},
		Columns: []query.Column{col(query.Col("c.a"))},
		From:    query.Table("c"),
	})
	wantSQL(t, sql, "WITH RECURSIVE c (a) AS (SELECT a FROM t UNION ALL SELECT c.a FROM c) SELECT c.a FROM c")
}

func TestCTEFailures(t *testing.T) {
	base := selectOf(col(query.Col("a")))
	nested := base
	nested.With = []query.CTE{{Name: "c", Query: base}}

	cases := map[string]struct {
		s    query.Select
		want string
	}{
		"missing name": {
			query.Select{
				With:    []query.CTE{{Query: base}},
				Columns: []query.Column{col(query.Col("a"))},
				From:    query.Table("c"),
			},
			"missing its name",
		},
		"missing query": {
			query.Select{
				With:    []query.CTE{{Name: "c"}},
				Columns: []query.Column{col(query.Col("a"))},
				From:    query.Table("c"),
			},
			"missing its query",
		},
		"WITH in derived table": {
			query.Select{
				Columns: []query.Column{col(query.Col("a"))},
				From:    query.Derived(nested).As("d"),
			},
			"outermost statement",
		},
	}
	for name, c := range cases {
		err := renderErr(t, c.s)
		if !errors.Is(err, query.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestSubqueryOrderRules(t *testing.T) {
	inner := selectOf(col(query.Col("a")))
	inner.OrderBy = []query.Order{{Expr: query.Col("a")}}

	err := renderErr(t, query.Select{
		Columns: []query.Column{col(query.Col("d.a"))},
		From:    query.Derived(inner).As("d"),
	})
	if !strings.Contains(err.Error(), "orders only when it pages") {
		t.Errorf("error = %v, want the subquery ordering rule", err)
	}

	paged := inner
	paged.Paging = &query.Paging{Offset: 5, Fetch: 5}
	sql, args := render(t, query.Select{
		Columns: []query.Column{col(query.Col("d.a"))},
		From:    query.Derived(paged).As("d"),
	})
	wantSQL(t, sql, "SELECT d.a FROM (SELECT a FROM t ORDER BY a OFFSET $1 ROWS FETCH NEXT $2 ROWS ONLY) d")
	wantArgs(t, args, 5, 5)
}

func TestPagingValidation(t *testing.T) {
	base := selectOf(col(query.Col("a")))
	ordered := base
	ordered.OrderBy = []query.Order{{Expr: query.Col("a")}}

	cases := map[string]struct {
		s    query.Select
		want string
	}{
		"without ORDER BY": {
			func() query.Select { s := base; s.Paging = &query.Paging{Fetch: 5}; return s }(),
			"paging requires ORDER BY",
		},
		"negative offset": {
			func() query.Select { s := ordered; s.Paging = &query.Paging{Offset: -1, Fetch: 5}; return s }(),
			"must not be negative",
		},
		"zero fetch": {
			func() query.Select { s := ordered; s.Paging = &query.Paging{}; return s }(),
			"at least 1",
		},
	}
	for name, c := range cases {
		err := renderErr(t, c.s)
		if !errors.Is(err, query.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestPagingRendererSeam(t *testing.T) {
	s := selectOf(col(query.Col("a")))
	s.OrderBy = []query.Order{{Expr: query.Col("a")}}
	s.Paging = &query.Paging{Offset: 50, Fetch: 25}

	sql, args, err := s.SQL(limitStub{})
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	wantSQL(t, sql, "SELECT a FROM t ORDER BY a LIMIT $1 OFFSET $2")
	wantArgs(t, args, 25, 50)
}

func TestPlaceholderSequencing(t *testing.T) {
	base := query.Select{
		Columns: []query.Column{query.Fn("CONCAT", query.Col("a"), "-").As("tag")},
		From:    query.Table("t"),
	}
	sql, args := render(t, query.Select{
		With:    []query.CTE{{Name: "c", Query: base}},
		Columns: []query.Column{col(query.Col("c.tag"))},
		From:    query.Table("c"),
		Where:   query.Col("c.tag").Like("x%"),
		OrderBy: []query.Order{{Expr: query.Col("c.tag")}},
		Paging:  &query.Paging{Offset: 10, Fetch: 5},
	})
	wantSQL(t, sql,
		"WITH c AS (SELECT CONCAT(a, $1) AS tag FROM t) SELECT c.tag FROM c "+
			"WHERE c.tag LIKE $2 ORDER BY c.tag OFFSET $3 ROWS FETCH NEXT $4 ROWS ONLY")
	wantArgs(t, args, "-", "x%", 10, 5)
}

func TestNilDialectPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("SQL(nil) did not panic")
		}
	}()
	_, _, _ = selectOf(col(query.Col("a"))).SQL(nil)
}
