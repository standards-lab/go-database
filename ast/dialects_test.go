package ast_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
)

// stub is the hermetic dialect every generation test renders through:
// $N placeholders, no error classification, no capabilities.
type stub struct{}

func (stub) Name() string             { return "stub" }
func (stub) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (stub) MapError(err error) error { return err }

// limitStub additionally implements ast.PagingRenderer, proving the
// dialect render seam: paging emits the extension's clause instead of the
// SQL:2008 form.
type limitStub struct{ stub }

func (limitStub) RenderPaging(w ast.Writer, offset, fetch int) {
	w.SQL("LIMIT ")
	w.Bind(fetch)
	w.SQL(" OFFSET ")
	w.Bind(offset)
}

// returningStub additionally implements ast.ReturningRenderer, proving the
// declared-native seam: a returning clause renders only through the
// capability.
type returningStub struct{ stub }

func (returningStub) RenderReturning(w ast.Writer, cols []ast.Column) {
	w.SQL("RETURNING ")
	for i, c := range cols {
		if i > 0 {
			w.SQL(", ")
		}
		w.Column(c)
	}
}

type renderable interface {
	Render(d database.Dialect) (ast.SQL, error)
}

func render(t *testing.T, q renderable) (string, []any) {
	t.Helper()
	s, err := q.Render(stub{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	return s.Text, s.Args
}

func renderErr(t *testing.T, q renderable) error {
	t.Helper()
	s, err := q.Render(stub{})
	if err == nil {
		t.Fatalf("Render() = %q, want error", s.Text)
	}
	return err
}

func wantSQL(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("sql = %q, want %q", got, want)
	}
}

func wantArgs(t *testing.T, got []any, want ...any) {
	t.Helper()
	if len(want) == 0 {
		if len(got) != 0 {
			t.Errorf("args = %v, want none", got)
		}
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

// selectOf is the minimal valid statement the node tests decorate.
func selectOf(cols ...ast.Column) ast.Select {
	return ast.Select{Columns: cols, From: ast.Table("t")}
}

func col(e ast.Expression) ast.Column { return ast.Column{Expr: e} }
