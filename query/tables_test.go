package query_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/query"
)

func fromOf(f query.TableRef) query.Select {
	return query.Select{
		Columns: []query.Column{col(query.Col("a"))},
		From:    f,
	}
}

func TestTableAliasRendersWithoutAS(t *testing.T) {
	sql, _ := render(t, fromOf(query.Table("organization").As("o")))
	wantSQL(t, sql, "SELECT a FROM organization o")
}

func TestEmptyTableNameFails(t *testing.T) {
	if err := renderErr(t, fromOf(query.Table(""))); !errors.Is(err, query.ErrInvalidStatement) {
		t.Errorf("error = %v, want ErrInvalidStatement", err)
	}
}

func TestZeroTableRefFails(t *testing.T) {
	err := renderErr(t, fromOf(query.TableRef{}))
	if !errors.Is(err, query.ErrInvalidStatement) || !strings.Contains(err.Error(), "missing FROM") {
		t.Errorf("error = %v, want ErrInvalidStatement with missing FROM", err)
	}
}

func TestDerivedTable(t *testing.T) {
	sql, _ := render(t, fromOf(query.Derived(selectOf(col(query.Col("a")))).As("d")))
	wantSQL(t, sql, "SELECT a FROM (SELECT a FROM t) d")
}

func TestDerivedTableFailures(t *testing.T) {
	for name, f := range map[string]query.TableRef{
		"missing alias": query.Derived(selectOf(col(query.Col("a")))),
		"missing query": query.Derived(nil).As("d"),
	} {
		if err := renderErr(t, fromOf(f)); !errors.Is(err, query.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}

func TestJoinKinds(t *testing.T) {
	left := query.Table("a")
	right := query.Table("b")
	on := query.Col("a.id").Eq(query.Col("b.a_id"))
	cases := map[string]struct {
		ref  query.TableRef
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
	joined := query.Table("a").Join(query.Table("b"), query.Col("a.id").Eq(query.Col("b.a_id")))
	for name, f := range map[string]query.TableRef{
		"aliased join": joined.As("j"),
		"missing ON":   query.Table("a").Join(query.Table("b"), query.Predicate{}),
	} {
		if err := renderErr(t, fromOf(f)); !errors.Is(err, query.ErrInvalidStatement) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement", name, err)
		}
	}
}
