package query

type tableNode interface {
	render(r *renderer)
}

// TableRef is a FROM clause element: a named table, an aliased derived
// table, or a join tree built by the join methods. The zero value fails the
// statement that renders it.
type TableRef struct {
	node tableNode
}

// Table references a named table or view, rendered verbatim.
func Table(name string) TableRef {
	return TableRef{
		namedTable{name: name},
	}
}

// Derived wraps a query as a derived table. It must be aliased with
// [TableRef.As] before it renders, and its query carries no WITH — the
// standard places WITH on the outermost statement; declare CTEs there.
func Derived(q Query) TableRef {
	return TableRef{
		derivedTable{q: q},
	}
}

// As sets the correlation name. Table aliases render without AS — the one
// spelling every engine accepts. Only a table or derived table takes an
// alias; aliasing a join fails the statement.
func (t TableRef) As(alias string) TableRef {
	switch n := t.node.(type) {
	case namedTable:
		n.alias = alias
		return TableRef{n}
	case derivedTable:
		n.alias = alias
		return TableRef{n}
	default:
		return TableRef{
			invalidRef("only a table or derived table takes an alias"),
		}
	}
}

// Join composes an INNER JOIN with the ON predicate.
func (t TableRef) Join(right TableRef, on Predicate) TableRef {
	return t.join("INNER JOIN", right, on)
}

// LeftJoin composes a LEFT JOIN with the ON predicate.
func (t TableRef) LeftJoin(right TableRef, on Predicate) TableRef {
	return t.join("LEFT JOIN", right, on)
}

// RightJoin composes a RIGHT JOIN with the ON predicate.
func (t TableRef) RightJoin(right TableRef, on Predicate) TableRef {
	return t.join("RIGHT JOIN", right, on)
}

// FullJoin composes a FULL JOIN with the ON predicate.
func (t TableRef) FullJoin(right TableRef, on Predicate) TableRef {
	return t.join("FULL JOIN", right, on)
}

func (t TableRef) join(kind string, right TableRef, on Predicate) TableRef {
	return TableRef{
		joinedTable{
			kind:  kind,
			left:  t.node,
			right: right.node,
			on:    on.node,
		},
	}
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
