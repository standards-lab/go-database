package query_test

import (
	"errors"
	"testing"

	"github.com/standards-lab/go-database/query"
)

func TestColSpellingsAreEquivalent(t *testing.T) {
	dotted, _ := render(t, selectOf(col(query.Col("o.name"))))
	parts, _ := render(t, selectOf(col(query.Col("o", "name"))))
	wantSQL(t, dotted, "SELECT o.name FROM t")
	if parts != dotted {
		t.Errorf("Col(\"o\", \"name\") = %q, Col(\"o.name\") = %q; want equal", parts, dotted)
	}
}

func TestColFailures(t *testing.T) {
	for name, e := range map[string]query.Expression{
		"no parts":   query.Col(),
		"empty part": query.Col("o", ""),
	} {
		if err := renderErr(t, selectOf(col(e))); !errors.Is(err, query.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestRaw(t *testing.T) {
	sql, args := render(t, selectOf(col(query.Raw("*"))))
	wantSQL(t, sql, "SELECT * FROM t")
	wantArgs(t, args)

	if err := renderErr(t, selectOf(col(query.Raw("")))); !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("empty Raw: error = %v, want ErrInvalidStatement", err)
	}
}

func TestValBinds(t *testing.T) {
	sql, args := render(t, selectOf(query.Val(42).As("answer")))
	wantSQL(t, sql, "SELECT $1 AS answer FROM t")
	wantArgs(t, args, 42)
}

func TestFnLiftsArguments(t *testing.T) {
	sql, args := render(t, selectOf(col(query.Fn("COALESCE", query.Col("nickname"), "unknown"))))
	wantSQL(t, sql, "SELECT COALESCE(nickname, $1) FROM t")
	wantArgs(t, args, "unknown")
}

func TestFnEmptyNameFails(t *testing.T) {
	if err := renderErr(t, selectOf(col(query.Fn("")))); !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestSelectAsExpression(t *testing.T) {
	max := selectOf(col(query.Fn("MAX", query.Col("age"))))
	sql, args := render(t, query.Select{
		Columns: []query.Column{col(query.Col("name"))},
		From:    query.Table("t"),
		Where:   query.Col("age").Eq(max),
	})
	wantSQL(t, sql, "SELECT name FROM t WHERE age = (SELECT MAX(age) FROM t)")
	wantArgs(t, args)
}

func TestZeroExpressionFails(t *testing.T) {
	if err := renderErr(t, selectOf(query.Column{})); !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}
