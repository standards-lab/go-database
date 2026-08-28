package ast

// SQL is one rendered statement — the render product every statement's
// Render method returns: the SQL text and its bound arguments in placeholder
// order. The execution layer runs SQL values; the operation layer's
// envelopes group them.
type SQL struct {
	Text string
	Args []any
}
