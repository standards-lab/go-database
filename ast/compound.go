package ast

import "github.com/standards-lab/go-database"

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

// Render renders the compound through the dialect, with the same contract
// as [Select.Render].
func (c Compound) Render(d database.Dialect) (SQL, error) {
	return renderSQL(d, c)
}

func (c Compound) queryExpr() {}

func (c Compound) render(r *renderer, mode queryMode) {
	if len(c.Queries) < 2 {
		r.fail("compound requires at least two queries")
		return
	}
	renderWith(r, c.With, mode)
	op := " " + string(c.Op)
	if c.All {
		op += " ALL"
	}
	op += " "
	for i, q := range c.Queries {
		if _, ok := q.(Select); !ok {
			r.fail("compound branches must be Select values")
			return
		}
		if i > 0 {
			r.write(op)
		}
		q.render(r, modeBranch)
	}
	renderTail(r, c.OrderBy, c.Paging, mode)
}
