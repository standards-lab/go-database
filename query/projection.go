package query

// Field is one projected field: the contract name directives reference, and
// the expression that computes it.
type Field struct {
	Name string
	Expr Expression
}

// Projection is a read model over one FROM: name-to-expression fields, one
// key field, and the CTEs the FROM references. It is plain data; every read
// against it goes through [Projection.Statements]. The key field is the
// identity a read pages by — it joins every ORDER BY as the tie-breaker, so
// offset paging is stable. CTEs declared here hoist to the top of both
// generated statements, the one placement every engine accepts.
type Projection struct {
	With   []CTE
	From   TableRef
	Key    Field
	Fields []Field
}
