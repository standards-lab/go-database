package query_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/query"
)

// stub is the hermetic dialect every generation test renders through:
// $N placeholders, no error classification.
type stub struct{}

func (stub) Name() string             { return "stub" }
func (stub) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }
func (stub) MapError(err error) error { return err }

// limitStub additionally implements query.PagingRenderer, proving the
// dialect render seam: paging emits the extension's clause instead of the
// SQL:2008 form.
type limitStub struct{ stub }

func (limitStub) RenderPaging(w query.Writer, offset, fetch int) {
	w.SQL("LIMIT ")
	w.Bind(fetch)
	w.SQL(" OFFSET ")
	w.Bind(offset)
}

type sqler interface {
	SQL(d database.Dialect) (string, []any, error)
}

func render(t *testing.T, q sqler) (string, []any) {
	t.Helper()
	sql, args, err := q.SQL(stub{})
	if err != nil {
		t.Fatalf("SQL() error: %v", err)
	}
	return sql, args
}

func renderErr(t *testing.T, q sqler) error {
	t.Helper()
	sql, _, err := q.SQL(stub{})
	if err == nil {
		t.Fatalf("SQL() = %q, want error", sql)
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
func selectOf(cols ...query.Column) query.Select {
	return query.Select{Columns: cols, From: query.Table("t")}
}

func col(e query.Expression) query.Column { return query.Column{Expr: e} }
