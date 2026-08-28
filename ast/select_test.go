package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

func TestSelectAllClauses(t *testing.T) {
	sql, args := render(t, ast.Select{
		Distinct: true,
		Columns: []ast.Column{
			col(ast.Col("kind")),
			ast.Fn("COUNT", ast.Raw("*")).As("total"),
		},
		From:    ast.Table("t"),
		Where:   ast.Col("age").Ge(21),
		GroupBy: []ast.Expression{ast.Col("kind")},
		Having:  ast.Fn("COUNT", ast.Raw("*")).Gt(1),
		OrderBy: []ast.Order{{Expr: ast.Col("kind")}},
		Paging:  &ast.Paging{Offset: 0, Fetch: 5},
	})
	wantSQL(t, sql,
		"SELECT DISTINCT kind, COUNT(*) AS total FROM t WHERE age >= $1 "+
			"GROUP BY kind HAVING COUNT(*) > $2 ORDER BY kind "+
			"OFFSET $3 ROWS FETCH NEXT $4 ROWS ONLY")
	wantArgs(t, args, 21, 1, 0, 5)
}

func TestEmptySelectListFails(t *testing.T) {
	err := renderErr(t, ast.Select{From: ast.Table("t")})
	if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), "empty select list") {
		t.Errorf("error = %v, want ErrInvalidStatement with empty select list", err)
	}
}

func TestMissingFromFails(t *testing.T) {
	err := renderErr(t, ast.Select{Columns: []ast.Column{col(ast.Col("a"))}})
	if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), "missing FROM") {
		t.Errorf("error = %v, want ErrInvalidStatement with missing FROM", err)
	}
}

func TestFirstFailureWins(t *testing.T) {
	err := renderErr(t, ast.Select{})
	if !strings.Contains(err.Error(), "empty select list") {
		t.Errorf("error = %v, want the first defect (empty select list)", err)
	}
}

func TestValidationPrecedesWith(t *testing.T) {
	base := selectOf(col(ast.Col("a")))
	err := renderErr(t, ast.Select{
		With: []ast.CTE{{Query: base}},
		From: ast.Table("c"),
	})
	if !strings.Contains(err.Error(), "empty select list") {
		t.Errorf("error = %v, want the statement's own defect before the CTE's", err)
	}
}

func TestOrderByDescending(t *testing.T) {
	sql, _ := render(t, ast.Select{
		Columns: []ast.Column{col(ast.Col("a"))},
		From:    ast.Table("t"),
		OrderBy: []ast.Order{{Expr: ast.Col("a")}, {Expr: ast.Col("b"), Descending: true}},
	})
	wantSQL(t, sql, "SELECT a FROM t ORDER BY a, b DESC")
}

func TestPagingValidation(t *testing.T) {
	base := selectOf(col(ast.Col("a")))
	ordered := base
	ordered.OrderBy = []ast.Order{{Expr: ast.Col("a")}}

	cases := map[string]struct {
		s    ast.Select
		want string
	}{
		"without ORDER BY": {
			func() ast.Select { s := base; s.Paging = &ast.Paging{Fetch: 5}; return s }(),
			"paging requires ORDER BY",
		},
		"negative offset": {
			func() ast.Select { s := ordered; s.Paging = &ast.Paging{Offset: -1, Fetch: 5}; return s }(),
			"must not be negative",
		},
		"zero fetch": {
			func() ast.Select { s := ordered; s.Paging = &ast.Paging{}; return s }(),
			"at least 1",
		},
	}
	for name, c := range cases {
		err := renderErr(t, c.s)
		if !errors.Is(err, ast.ErrInvalidStatement) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want ErrInvalidStatement containing %q", name, err, c.want)
		}
	}
}

func TestPagingRendererSeam(t *testing.T) {
	s := selectOf(col(ast.Col("a")))
	s.OrderBy = []ast.Order{{Expr: ast.Col("a")}}
	s.Paging = &ast.Paging{Offset: 50, Fetch: 25}

	res, err := s.Render(limitStub{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	wantSQL(t, res.Text, "SELECT a FROM t ORDER BY a LIMIT $1 OFFSET $2")
	wantArgs(t, res.Args, 25, 50)
}
