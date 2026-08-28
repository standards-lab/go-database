package ast

// Writer is the surface a dialect render extension writes into: literal SQL
// text through SQL, bound arguments through Bind, and select-list columns
// through Column — so an extension composes its clause without reaching the
// package's internals, and bound arguments stay in placeholder order.
type Writer struct {
	r *renderer
}

// SQL writes literal SQL text.
func (w Writer) SQL(s string) {
	w.r.write(s)
}

// Bind binds a value and writes its placeholder.
func (w Writer) Bind(v any) {
	w.r.bind(v)
}

// Column renders one select-list column: its expression, and AS with the
// alias when one is set.
func (w Writer) Column(c Column) {
	w.r.expr(c.Expr.node)
	if c.Alias != "" {
		w.r.write(" AS " + c.Alias)
	}
}

// PagingRenderer is the render override for an engine that lacks the
// SQL:2008 OFFSET/FETCH form. A provider's dialect opts in by implementing
// it and writes its engine's paging clause — no leading space, offset and
// fetch bound through the writer rather than interpolated. Absent, the
// standard form renders: this is the capability pattern's override
// direction, where standard emission stays the default.
type PagingRenderer interface {
	RenderPaging(w Writer, offset, fetch int)
}

// ReturningRenderer is the capability for the declared-native returning
// clause on the write statements — an engine's way of handing back the rows
// a write affected, which has no standard emission. A provider's dialect
// opts in and writes its engine's clause at the statement tail — no leading
// space, columns through [Writer.Column]. Absent, a statement carrying a
// Returning list fails with [UnsupportedFeatureError]: the capability
// pattern's native-only direction, failing typed rather than falling back.
type ReturningRenderer interface {
	RenderReturning(w Writer, cols []Column)
}
