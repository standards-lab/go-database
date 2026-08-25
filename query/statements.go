package query

import "github.com/standards-lab/go-database"

// Query is a renderable query: [Select] or [Compound]. The interface is
// sealed; the two implementations are the vocabulary's statement level.
type Query interface {
	render(r *renderer, mode queryMode)
}

// CTE is one common table expression in a statement's WITH list. Recursive
// marks it for WITH RECURSIVE — its query is then a [Compound] whose first
// branch is the anchor. Columns optionally names the CTE's column list,
// which a recursive CTE usually needs.
type CTE struct {
	Name      string
	Columns   []string
	Recursive bool
	Query     Query
}

// Order is one ORDER BY key. Ascending is the default; Descending renders
// DESC.
type Order struct {
	Expr       Expression
	Descending bool
}

// Paging is the SQL:2008 offset/fetch pair, rendered as bound arguments:
// OFFSET ? ROWS FETCH NEXT ? ROWS ONLY. Paging always requires ORDER BY. An
// engine that lacks the standard form renders through [PagingRenderer].
type Paging struct {
	Offset int
	Fetch  int
}

// Select is a query specification as a value: build it as a struct literal
// and render it with [Select.SQL]. Zero fields are absent clauses. WITH is
// valid only on the outermost statement, and a subquery orders only when it
// pages.
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

// SQL renders the statement through the dialect, returning the text and the
// bound arguments in placeholder order. A structural defect wraps
// [ErrInvalidStatement]; a nil dialect panics as a wiring defect.
func (s Select) SQL(d database.Dialect) (string, []any, error) {
	return renderSQL(d, s)
}

func (s Select) render(r *renderer, mode queryMode) {
	renderWith(r, s.With, mode)
	r.write("SELECT ")
	if s.Distinct {
		r.write("DISTINCT ")
	}
	if len(s.Columns) == 0 {
		r.fail("empty select list")
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
	if s.From.node == nil {
		r.fail("missing FROM")
	} else {
		s.From.node.render(r)
	}
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

// SetOp is a compound query's set operator.
type SetOp string

const (
	SetUnion     SetOp = "UNION"
	SetIntersect SetOp = "INTERSECT"
	SetExcept    SetOp = "EXCEPT"
)

// Compound joins two or more Select branches with one set operator.
// Branches render without parentheses — the one form every engine accepts —
// so each compound carries a single operator and deliberate nesting goes
// through a derived table. ORDER BY and paging attach to the compound;
// branches carry neither, nor WITH.
type Compound struct {
	With    []CTE
	Op      SetOp
	All     bool
	Queries []Query
	OrderBy []Order
	Paging  *Paging
}

// Union composes a UNION of the queries.
func Union(queries ...Query) Compound {
	return Compound{
		Op:      SetUnion,
		Queries: queries,
	}
}

// UnionAll composes a UNION ALL of the queries.
func UnionAll(queries ...Query) Compound {
	return Compound{
		Op:      SetUnion,
		All:     true,
		Queries: queries,
	}
}

// Intersect composes an INTERSECT of the queries.
func Intersect(queries ...Query) Compound {
	return Compound{
		Op:      SetIntersect,
		Queries: queries,
	}
}

// IntersectAll composes an INTERSECT ALL of the queries.
func IntersectAll(queries ...Query) Compound {
	return Compound{
		Op:      SetIntersect,
		All:     true,
		Queries: queries,
	}
}

// Except composes an EXCEPT of the queries.
func Except(queries ...Query) Compound {
	return Compound{
		Op:      SetExcept,
		Queries: queries,
	}
}

// ExceptAll composes an EXCEPT ALL of the queries.
func ExceptAll(queries ...Query) Compound {
	return Compound{
		Op:      SetExcept,
		All:     true,
		Queries: queries,
	}
}

// SQL renders the compound through the dialect, with the same contract as
// [Select.SQL].
func (c Compound) SQL(d database.Dialect) (string, []any, error) {
	return renderSQL(d, c)
}

func (c Compound) render(r *renderer, mode queryMode) {
	renderWith(r, c.With, mode)
	if len(c.Queries) < 2 {
		r.fail("compound requires at least two queries")
		return
	}
	op := " " + string(c.Op)
	if c.All {
		op += " ALL"
	}
	op += " "
	for i, q := range c.Queries {
		if i > 0 {
			r.write(op)
		}
		if _, ok := q.(Select); !ok {
			r.fail("compound branches must be Select values")
			return
		}
		q.render(r, modeBranch)
	}
	renderTail(r, c.OrderBy, c.Paging, mode)
}

// renderSQL renders a statement at the outermost position. A nil dialect is
// a wiring defect at the composition root, not a runtime condition.
func renderSQL(d database.Dialect, q Query) (string, []any, error) {
	if d == nil {
		panic("query: nil dialect")
	}
	r := &renderer{dialect: d}
	q.render(r, modeTop)
	if r.err != nil {
		return "", nil, r.err
	}
	return r.sb.String(), r.args, nil
}

func renderTail(r *renderer, orders []Order, paging *Paging, mode queryMode) {
	if mode == modeBranch && (len(orders) > 0 || paging != nil) {
		r.fail("ORDER BY and paging are not allowed on a compound branch; set them on the compound")
		return
	}
	if paging != nil && len(orders) == 0 {
		r.fail("paging requires ORDER BY")
		return
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
		if paging.Offset < 0 {
			r.fail("paging offset must not be negative")
			return
		}
		if paging.Fetch < 1 {
			r.fail("paging fetch must be at least 1")
			return
		}
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
		if i > 0 {
			r.write(", ")
		}
		if c.Name == "" {
			r.fail("CTE missing its name")
			return
		}
		if c.Query == nil {
			r.fail("CTE missing its query")
			return
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
