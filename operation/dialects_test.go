package operation_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/standards-lab/go-database/ast"
)

// stub is the hermetic dialect the contract tests lower through: $N
// placeholders, no error classification, no capabilities.
type stub struct{}

func (stub) Name() string             { return "stub" }
func (stub) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (stub) MapError(err error) error { return err }

// returningStub additionally implements ast.ReturningRenderer, so the
// command shapes that require the capability can lower.
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
