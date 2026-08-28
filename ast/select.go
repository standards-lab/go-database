package ast

import "github.com/standards-lab/go-database"

// Select is a query specification as a value: build it as a struct literal
// and render it with [Select.Render]. Zero fields are absent clauses. WITH
// is valid only on the outermost statement, and a subquery orders only when
// it pages.
type Select struct {
	With     []CTE
	Distinct bool
	Columns  []Column
	From     TableRef
	Where    Predicate
	GroupBy  []Expression
	Having   Predicate
	OrderBy  []Order
	Paging   *Paging
}

// Render renders the statement through the dialect. A structural defect
// wraps [ErrInvalidStatement]; a nil dialect panics as a wiring defect.
func (s Select) Render(d database.Dialect) (SQL, error) {
	return renderSQL(d, s)
}

func (s Select) queryExpr() {}

func (s Select) render(r *renderer, mode queryMode) {
	if len(s.Columns) == 0 {
		r.fail("empty select list")
		return
	}
	if s.From.node == nil {
		r.fail("missing FROM")
		return
	}
	renderWith(r, s.With, mode)
	r.write("SELECT ")
	if s.Distinct {
		r.write("DISTINCT ")
	}

	for i, c := range s.Columns {
		if i > 0 {
			r.write(", ")
		}
		r.expr(c.Expr.node)
		if c.Alias != "" {
			r.write(" AS " + c.Alias)
		}
	}
	r.write(" FROM ")
	s.From.node.render(r)
	if s.Where.node != nil {
		r.write(" WHERE ")
		r.expr(s.Where.node)
	}
	if len(s.GroupBy) > 0 {
		r.write(" GROUP BY ")
		for i, g := range s.GroupBy {
			if i > 0 {
				r.write(", ")
			}
			r.expr(g.node)
		}
	}
	if s.Having.node != nil {
		r.write(" HAVING ")
		r.expr(s.Having.node)
	}
	renderTail(r, s.OrderBy, s.Paging, mode)
}
