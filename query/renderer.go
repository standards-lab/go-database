package query

import (
	"fmt"
	"strings"

	"github.com/standards-lab/go-database"
)

// PagingRenderer is the dialect render extension for an engine that lacks
// the SQL:2008 OFFSET/FETCH form. A provider's dialect opts in by
// implementing it and writes its engine's paging clause — no leading space,
// offset and fetch bound through the writer rather than interpolated.
// Absent, the standard form renders. This is the vocabulary's extension
// pattern: where an engine lacks a standard form, the package defines an
// optional interface at the divergent render site and standard emission
// stays the default.
type PagingRenderer interface {
	RenderPaging(w Writer, offset, fetch int)
}

// queryMode is the rendering position of a query, carrying the standard's
// placement rules: WITH and a bare ORDER BY belong to the outermost
// statement, and a compound branch carries no tail clauses at all.
type queryMode int

const (
	modeTop queryMode = iota
	modeSub
	modeBranch
)

// renderer accumulates one statement in a single pass: the SQL text, the
// bound arguments in placeholder order, and the first structural defect
// found. Rendering never stops early; a failed render returns its error and
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

// bind appends a bound argument and writes the dialect's placeholder for its
// 1-based position.
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

// Writer is the surface a dialect render extension writes into: literal SQL
// text through SQL, bound arguments through Bind.
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
