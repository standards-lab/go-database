package ast

import (
	"fmt"
	"strings"

	"github.com/standards-lab/go-database"
)

// queryMode is the rendering position of a statement, carrying the
// standard's placement rules: WITH and a bare ORDER BY belong to the
// outermost statement, a compound branch carries no tail clauses at all,
// and a write statement renders only at the top.
type queryMode int

const (
	modeTop queryMode = iota
	modeSub
	modeBranch
)

// renderable is the internal statement contract every statement implements
// — the five statement types render through it. The public [Query]
// interface narrows it to the query expressions.
type renderable interface {
	render(r *renderer, mode queryMode)
}

// renderer accumulates one statement in a single pass: the SQL text, the
// bound arguments in placeholder order, and the first defect found.
// Rendering never stops early; a failed render returns its error and
// discards the text.
type renderer struct {
	sb      strings.Builder
	dialect database.Dialect
	args    []any
	err     error
}

func (r *renderer) write(s string) {
	r.sb.WriteString(s)
}

func (r *renderer) bind(v any) {
	r.args = append(r.args, v)
	r.sb.WriteString(r.dialect.Placeholder(len(r.args)))
}

// expr renders a node, failing the statement on the zero Expression.
func (r *renderer) expr(n exprNode) {
	if n == nil {
		r.fail("empty expression")
		return
	}
	n.render(r)
}

// fail records the first structural defect, wrapped around
// [ErrInvalidStatement].
func (r *renderer) fail(msg string) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %s", ErrInvalidStatement, msg)
	}
}

// failErr records a first error that is not an invalid-statement defect —
// the statement is sound, something else refuses it.
func (r *renderer) failErr(err error) {
	if r.err == nil {
		r.err = err
	}
}

// renderSQL renders a statement at the outermost position. A nil dialect is
// a wiring defect at the composition root, not a runtime condition.
func renderSQL(d database.Dialect, q renderable) (SQL, error) {
	if d == nil {
		panic("ast: nil dialect")
	}
	r := &renderer{dialect: d}
	q.render(r, modeTop)
	if r.err != nil {
		return SQL{}, r.err
	}
	return SQL{Text: r.sb.String(), Args: r.args}, nil
}

func renderTail(r *renderer, orders []Order, paging *Paging, mode queryMode) {
	if mode == modeBranch && (len(orders) > 0 || paging != nil) {
		r.fail("ORDER BY and paging are not allowed on a compound branch; set them on the compound")
		return
	}
	if paging != nil {
		if len(orders) == 0 {
			r.fail("paging requires ORDER BY")
			return
		}
		if paging.Offset < 0 {
			r.fail("paging offset must not be negative")
			return
		}
		if paging.Fetch < 1 {
			r.fail("paging fetch must be at least 1")
			return
		}
	}
	if mode == modeSub && len(orders) > 0 && paging == nil {
		r.fail("a subquery orders only when it pages")
		return
	}
	if len(orders) > 0 {
		r.write(" ORDER BY ")
		for i, o := range orders {
			if i > 0 {
				r.write(", ")
			}
			r.expr(o.Expr.node)
			if o.Descending {
				r.write(" DESC")
			}
		}
	}
	if paging != nil {
		if pr, ok := r.dialect.(PagingRenderer); ok {
			r.write(" ")
			pr.RenderPaging(Writer{r}, paging.Offset, paging.Fetch)
			return
		}
		r.write(" OFFSET ")
		r.bind(paging.Offset)
		r.write(" ROWS FETCH NEXT ")
		r.bind(paging.Fetch)
		r.write(" ROWS ONLY")
	}
}

func renderWith(r *renderer, ctes []CTE, mode queryMode) {
	if len(ctes) == 0 {
		return
	}
	if mode != modeTop {
		r.fail("WITH belongs to the outermost statement")
		return
	}
	r.write("WITH ")
	for _, c := range ctes {
		if c.Recursive {
			r.write("RECURSIVE ")
			break
		}
	}
	for i, c := range ctes {
		if c.Name == "" {
			r.fail("CTE missing its name")
			return
		}
		if c.Query == nil {
			r.fail("CTE missing its query")
			return
		}
		if i > 0 {
			r.write(", ")
		}
		r.write(c.Name)
		if len(c.Columns) > 0 {
			r.write(" (")
			for j, col := range c.Columns {
				if j > 0 {
					r.write(", ")
				}
				r.write(col)
			}
			r.write(")")
		}
		r.write(" AS (")
		c.Query.render(r, modeSub)
		r.write(")")
	}
	r.write(" ")
}

// renderReturning is the declared-native clause's render site: through the
// [ReturningRenderer] capability when the dialect opts in, a typed
// [UnsupportedFeatureError] when it doesn't — never a silent fallback.
func renderReturning(r *renderer, cols []Column) {
	if len(cols) == 0 {
		return
	}
	rr, ok := r.dialect.(ReturningRenderer)
	if !ok {
		r.failErr(&UnsupportedFeatureError{
			Feature: FeatureReturning,
			Dialect: r.dialect.Name(),
		})
		return
	}
	r.write(" ")
	rr.RenderReturning(Writer{r}, cols)
}
