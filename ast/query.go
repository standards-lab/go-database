package ast

// Query is a query expression: [Select] or [Compound], the statements SQL
// grammar admits inside a CTE, a derived table, a subquery operand, and
// INSERT ... SELECT. The interface is sealed by an unexported marker, so
// those positions accept exactly the query expressions at compile time; the
// write statements render at the outermost position only and never
// implement it.
type Query interface {
	renderable
	queryExpr()
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
