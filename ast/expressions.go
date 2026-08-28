package ast

import "strings"

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

// Empty reports whether the expression is the zero value. Layers above the
// package use it to validate their contracts before rendering.
func (e Expression) Empty() bool {
	return e.node == nil
}

// Column is one select-list entry: an expression and its optional alias.
type Column struct {
	Expr  Expression
	Alias string
}
