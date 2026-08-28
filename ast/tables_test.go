package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func fromOf(f ast.TableRef) ast.Select {
	return ast.Select{
		Columns: []ast.Column{col(ast.Col("a"))},
		From:    f,
	}
}

func TestTableAliasRendersWithoutAS(t *testing.T) {
	sql, _ := render(t, fromOf(ast.Table("organization").As("o")))
	wantSQL(t, sql, "SELECT a FROM organization o")
}

func TestEmptyTableNameFails(t *testing.T) {
	if err := renderErr(t, fromOf(ast.Table(""))); !errors.Is(err, ast.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestZeroTableRefFails(t *testing.T) {
	err := renderErr(t, fromOf(ast.TableRef{}))
	if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), "missing FROM") {
		t.Errorf("error = %v, want ErrInvalidStatement with missing FROM", err)
	}
}

func TestDerivedTable(t *testing.T) {
	sql, _ := render(t, fromOf(ast.Derived(selectOf(col(ast.Col("a")))).As("d")))
	wantSQL(t, sql, "SELECT a FROM (SELECT a FROM t) d")
}

func TestDerivedTableFailures(t *testing.T) {
	for name, f := range map[string]ast.TableRef{
		"missing alias": ast.Derived(selectOf(col(ast.Col("a")))),
		"missing query": ast.Derived(nil).As("d"),
	} {
		if err := renderErr(t, fromOf(f)); !errors.Is(err, ast.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestJoinKinds(t *testing.T) {
	left := ast.Table("a")
	right := ast.Table("b")
	on := ast.Col("a.id").Eq(ast.Col("b.a_id"))
	cases := map[string]struct {
		ref  ast.TableRef
		kind string
	}{
		"Join":      {left.Join(right, on), "INNER JOIN"},
		"LeftJoin":  {left.LeftJoin(right, on), "LEFT JOIN"},
		"RightJoin": {left.RightJoin(right, on), "RIGHT JOIN"},
		"FullJoin":  {left.FullJoin(right, on), "FULL JOIN"},
	}
	for name, c := range cases {
		sql, _ := render(t, fromOf(c.ref))
		wantSQL(t, sql, "SELECT a FROM a "+c.kind+" b ON a.id = b.a_id")
		_ = name
	}
}

func TestJoinFailures(t *testing.T) {
	joined := ast.Table("a").Join(ast.Table("b"), ast.Col("a.id").Eq(ast.Col("b.a_id")))
	for name, f := range map[string]ast.TableRef{
		"aliased join": joined.As("j"),
		"missing ON":   ast.Table("a").Join(ast.Table("b"), ast.Predicate{}),
	} {
		if err := renderErr(t, fromOf(f)); !errors.Is(err, ast.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}
