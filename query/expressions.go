package query

import "strings"

type exprNode interface {
	render(r *renderer)
}

// operand is implemented by the value types that wrap a node — [Expression]
// and [Predicate] — so [lift] accepts them without naming them.
type operand interface {
	operand() exprNode
}

// Expression is one standard-SQL expression: a column reference, a bound
// value, a function call, a raw fragment, or a subquery. The zero value is
// empty and fails the statement that renders it. The comparison methods
// produce [Predicate] values; [Expression.As] names it as a select-list
// column.
type Expression struct{ node exprNode }

func (e Expression) operand() exprNode {
	return e.node
}

// Col references a column by its identifier parts. Each part may itself be
// dotted — Col("o.name") and Col("o", "name") are the same reference — so
// the structure survives for a dialect quoting extension while call sites
// keep reading like SQL. Parts render verbatim, dot-joined; an identifier
// containing a literal dot goes through [Raw]. An empty part fails the
// statement.
func Col(parts ...string) Expression {
	var names []string
	for _, p := range parts {
		names = append(names, strings.Split(p, ".")...)
	}
	return Expression{colNode(names)}
}

// Raw is the escape hatch: a consumer-authored SQL fragment rendered
// verbatim. It is the deliberate seam for SQL the vocabulary does not
// compose; never route request-derived text through it.
func Raw(sql string) Expression {
	return Expression{rawNode(sql)}
}

// Val binds a value explicitly. Comparison operands lift plain Go values on
// their own; Val is for a bound value standing alone as an expression, such
// as a constant select-list column.
func Val(v any) Expression {
	return Expression{valueNode{v}}
}

// Fn composes a function call. Arguments lift like comparison operands —
// expressions render in place, plain values bind — so Fn("COUNT", Raw("*"))
// and Fn("COALESCE", Col("nickname"), "") both hold.
func Fn(name string, args ...any) Expression {
	nodes := make([]exprNode, len(args))
	for i, a := range args {
		nodes[i] = lift(a)
	}
	return Expression{
		fnNode{
			name: name,
			args: nodes,
		},
	}
}

// As names the expression as a select-list column.
func (e Expression) As(alias string) Column {
	return Column{Expr: e, Alias: alias}
}

// Column is one select-list entry: an expression and its optional alias.
type Column struct {
	Expr  Expression
	Alias string
}

// lift turns an operand into a node: expressions and predicates pass
// through, a [Query] becomes a parenthesized subquery, and any other Go
// value binds as a parameter.
func lift(v any) exprNode {
	switch x := v.(type) {
	case operand:
		return x.operand()
	case Query:
		return subqueryNode{x}
	default:
		return valueNode{v}
	}
}

type colNode []string

func (n colNode) render(r *renderer) {
	if len(n) == 0 {
		r.fail("empty column reference")
		return
	}
	for i, part := range n {
		if part == "" {
			r.fail("empty column reference part")
			return
		}
		if i > 0 {
			r.write(".")
		}
		r.write(part)
	}
}

type rawNode string

func (n rawNode) render(r *renderer) {
	if n == "" {
		r.fail("empty raw fragment")
		return
	}
	r.write(string(n))
}

type valueNode struct {
	v any
}

func (n valueNode) render(r *renderer) {
	r.bind(n.v)
}

type fnNode struct {
	name string
	args []exprNode
}

func (n fnNode) render(r *renderer) {
	if n.name == "" {
		r.fail("empty function name")
		return
	}
	r.write(n.name)
	r.write("(")
	for i, a := range n.args {
		if i > 0 {
			r.write(", ")
		}
		r.expr(a)
	}
	r.write(")")
}

type subqueryNode struct{ q Query }

func (n subqueryNode) render(r *renderer) {
	r.write("(")
	n.q.render(r, modeSub)
	r.write(")")
}
