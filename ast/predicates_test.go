package ast_test

import (
	"errors"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func whereOf(p ast.Predicate) ast.Select {
	return ast.Select{
		Columns: []ast.Column{col(ast.Col("a"))},
		From:    ast.Table("t"),
		Where:   p,
	}
}

func TestComparisonSuite(t *testing.T) {
	age := ast.Col("age")
	cases := map[string]struct {
		pred ast.Predicate
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
	sql, args := render(t, whereOf(ast.Col("name").Like("ada%")))
	wantSQL(t, sql, "SELECT a FROM t WHERE name LIKE $1")
	wantArgs(t, args, "ada%")
}

func TestComparisonExpressionOperand(t *testing.T) {
	sql, args := render(t, whereOf(ast.Col("l.depth").Lt(ast.Col("o.max_depth"))))
	wantSQL(t, sql, "SELECT a FROM t WHERE l.depth < o.max_depth")
	wantArgs(t, args)
}

func TestInValues(t *testing.T) {
	sql, args := render(t, whereOf(ast.Col("kind").In("unit", "agency")))
	wantSQL(t, sql, "SELECT a FROM t WHERE kind IN ($1, $2)")
	wantArgs(t, args, "unit", "agency")
}

func TestInSubquery(t *testing.T) {
	sub := selectOf(col(ast.Col("id")))
	sql, args := render(t, whereOf(ast.Col("id").In(sub)))
	wantSQL(t, sql, "SELECT a FROM t WHERE id IN (SELECT id FROM t)")
	wantArgs(t, args)
}

func TestInEmptyFails(t *testing.T) {
	if err := renderErr(t, whereOf(ast.Col("id").In())); !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestNullTests(t *testing.T) {
	sql, _ := render(t, whereOf(ast.Col("parent_id").IsNull()))
	wantSQL(t, sql, "SELECT a FROM t WHERE parent_id IS NULL")

	sql, _ = render(t, whereOf(ast.Col("parent_id").IsNotNull()))
	wantSQL(t, sql, "SELECT a FROM t WHERE parent_id IS NOT NULL")
}

func TestAndCollapsesAndDropsEmpty(t *testing.T) {
	one := ast.Col("a").Eq(1)
	two := ast.Col("b").Eq(2)

	sql, _ := render(t, whereOf(ast.And()))
	wantSQL(t, sql, "SELECT a FROM t")

	sql, _ = render(t, whereOf(ast.And(one)))
	wantSQL(t, sql, "SELECT a FROM t WHERE a = $1")

	sql, _ = render(t, whereOf(ast.And(one, ast.Predicate{}, two)))
	wantSQL(t, sql, "SELECT a FROM t WHERE (a = $1 AND b = $2)")
}

func TestNestedLogic(t *testing.T) {
	sql, args := render(t, whereOf(ast.And(
		ast.Col("a").Eq(1),
		ast.Or(
			ast.Col("b").Eq(2),
			ast.Col("c").Eq(3),
		),
	)))
	wantSQL(t, sql, "SELECT a FROM t WHERE (a = $1 AND (b = $2 OR c = $3))")
	wantArgs(t, args, 1, 2, 3)
}

func TestNot(t *testing.T) {
	sql, _ := render(t, whereOf(ast.Not(ast.Col("a").Eq(1))))
	wantSQL(t, sql, "SELECT a FROM t WHERE NOT (a = $1)")

	if err := renderErr(t, whereOf(ast.Not(ast.Predicate{}))); !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("Not(empty): error = %v, want ErrInvalidStatement", err)
	}
}
