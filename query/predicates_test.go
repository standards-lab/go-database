package query_test

import (
	"errors"
	"testing"

	"github.com/standards-lab/go-database/query"
)

func whereOf(p query.Predicate) query.Select {
	return query.Select{
		Columns: []query.Column{col(query.Col("a"))},
		From:    query.Table("t"),
		Where:   p,
	}
}

func TestComparisonSuite(t *testing.T) {
	age := query.Col("age")
	cases := map[string]struct {
		pred query.Predicate
		op   string
	}{
		"Eq": {age.Eq(21), "="},
		"Ne": {age.Ne(21), "<>"},
		"Gt": {age.Gt(21), ">"},
		"Ge": {age.Ge(21), ">="},
		"Lt": {age.Lt(21), "<"},
		"Le": {age.Le(21), "<="},
	}
	for name, c := range cases {
		sql, args := render(t, whereOf(c.pred))
		wantSQL(t, sql, "SELECT a FROM t WHERE age "+c.op+" $1")
		wantArgs(t, args, 21)
		_ = name
	}
}

func TestLike(t *testing.T) {
	sql, args := render(t, whereOf(query.Col("name").Like("ada%")))
	wantSQL(t, sql, "SELECT a FROM t WHERE name LIKE $1")
	wantArgs(t, args, "ada%")
}

func TestComparisonExpressionOperand(t *testing.T) {
	sql, args := render(t, whereOf(query.Col("l.depth").Lt(query.Col("o.max_depth"))))
	wantSQL(t, sql, "SELECT a FROM t WHERE l.depth < o.max_depth")
	wantArgs(t, args)
}

func TestInValues(t *testing.T) {
	sql, args := render(t, whereOf(query.Col("kind").In("unit", "agency")))
	wantSQL(t, sql, "SELECT a FROM t WHERE kind IN ($1, $2)")
	wantArgs(t, args, "unit", "agency")
}

func TestInSubquery(t *testing.T) {
	sub := selectOf(col(query.Col("id")))
	sql, args := render(t, whereOf(query.Col("id").In(sub)))
	wantSQL(t, sql, "SELECT a FROM t WHERE id IN (SELECT id FROM t)")
	wantArgs(t, args)
}

func TestInEmptyFails(t *testing.T) {
	if err := renderErr(t, whereOf(query.Col("id").In())); !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestNullTests(t *testing.T) {
	sql, _ := render(t, whereOf(query.Col("parent_id").IsNull()))
	wantSQL(t, sql, "SELECT a FROM t WHERE parent_id IS NULL")

	sql, _ = render(t, whereOf(query.Col("parent_id").IsNotNull()))
	wantSQL(t, sql, "SELECT a FROM t WHERE parent_id IS NOT NULL")
}

func TestAndCollapsesAndDropsEmpty(t *testing.T) {
	one := query.Col("a").Eq(1)
	two := query.Col("b").Eq(2)

	sql, _ := render(t, whereOf(query.And()))
	wantSQL(t, sql, "SELECT a FROM t")

	sql, _ = render(t, whereOf(query.And(one)))
	wantSQL(t, sql, "SELECT a FROM t WHERE a = $1")

	sql, _ = render(t, whereOf(query.And(one, query.Predicate{}, two)))
	wantSQL(t, sql, "SELECT a FROM t WHERE (a = $1 AND b = $2)")
}

func TestNestedLogic(t *testing.T) {
	sql, args := render(t, whereOf(query.And(
		query.Col("a").Eq(1),
		query.Or(
			query.Col("b").Eq(2),
			query.Col("c").Eq(3),
		),
	)))
	wantSQL(t, sql, "SELECT a FROM t WHERE (a = $1 AND (b = $2 OR c = $3))")
	wantArgs(t, args, 1, 2, 3)
}

func TestNot(t *testing.T) {
	sql, _ := render(t, whereOf(query.Not(query.Col("a").Eq(1))))
	wantSQL(t, sql, "SELECT a FROM t WHERE NOT (a = $1)")

	if err := renderErr(t, whereOf(query.Not(query.Predicate{}))); !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("Not(empty): error = %v, want ErrInvalidStatement", err)
	}
}
