package ast

// The render tree. Every public grammar value wraps one of these nodes; the
// grammar files define the vocabulary, this file and renderer.go are the
// machine it drives. Nothing here is reachable outside the package.

type exprNode interface {
	render(r *renderer)
}

// operand is implemented by the value types that wrap a node —
// [Expression] and [Predicate] — so [lift] accepts them without naming
// them.
type operand interface {
	operand() exprNode
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

type compareNode struct {
	op          string
	left, right exprNode
}

func (n compareNode) render(r *renderer) {
	r.expr(n.left)
	r.write(" " + n.op + " ")
	r.expr(n.right)
}

type nullNode struct {
	expr exprNode
	not  bool
}

func (n nullNode) render(r *renderer) {
	r.expr(n.expr)
	if n.not {
		r.write(" IS NOT NULL")
	} else {
		r.write(" IS NULL")
	}
}

type inNode struct {
	expr  exprNode
	items []exprNode
	sub   Query
}

func (n inNode) render(r *renderer) {
	if n.sub == nil && len(n.items) == 0 {
		r.fail("empty IN list")
		return
	}
	r.expr(n.expr)
	r.write(" IN (")
	if n.sub != nil {
		n.sub.render(r, modeSub)
	} else {
		for i, item := range n.items {
			if i > 0 {
				r.write(", ")
			}
			r.expr(item)
		}
	}
	r.write(")")
}

type logicNode struct {
	op    string
	preds []exprNode
}

func (n logicNode) render(r *renderer) {
	r.write("(")
	for i, p := range n.preds {
		if i > 0 {
			r.write(" " + n.op + " ")
		}
		r.expr(p)
	}
	r.write(")")
}

type notNode struct{ pred exprNode }

func (n notNode) render(r *renderer) {
	r.write("NOT (")
	r.expr(n.pred)
	r.write(")")
}

type tableNode interface {
	render(r *renderer)
}

type namedTable struct {
	name  string
	alias string
}

func (n namedTable) render(r *renderer) {
	if n.name == "" {
		r.fail("empty table name")
		return
	}
	r.write(n.name)
	if n.alias != "" {
		r.write(" " + n.alias)
	}
}

type derivedTable struct {
	q     Query
	alias string
}

func (n derivedTable) render(r *renderer) {
	if n.q == nil {
		r.fail("derived table missing its query")
		return
	}
	if n.alias == "" {
		r.fail("derived table requires an alias")
		return
	}
	r.write("(")
	n.q.render(r, modeSub)
	r.write(") " + n.alias)
}

type joinedTable struct {
	kind        string
	left, right tableNode
	on          exprNode
}

func (n joinedTable) render(r *renderer) {
	if n.left == nil || n.right == nil {
		r.fail("join missing a side")
		return
	}
	if n.on == nil {
		r.fail("join missing its ON predicate")
		return
	}
	n.left.render(r)
	r.write(" " + n.kind + " ")
	n.right.render(r)
	r.write(" ON ")
	r.expr(n.on)
}

type invalidRef string

func (n invalidRef) render(r *renderer) {
	r.fail(string(n))
}
