package ast

// Predicate is a boolean expression: a comparison, a null test, a
// membership test, or a combination. The zero value is empty — an absent
// WHERE or HAVING — and [And] and [Or] drop empty operands, so optional
// conditions compose without special cases.
type Predicate struct{ node exprNode }

func (p Predicate) operand() exprNode { return p.node }

// Eq compares for equality; the operand lifts like every comparison operand.
func (e Expression) Eq(v any) Predicate { return compare("=", e, v) }

// Ne compares for inequality.
func (e Expression) Ne(v any) Predicate { return compare("<>", e, v) }

// Gt compares greater-than.
func (e Expression) Gt(v any) Predicate { return compare(">", e, v) }

// Ge compares greater-than-or-equal.
func (e Expression) Ge(v any) Predicate { return compare(">=", e, v) }

// Lt compares less-than.
func (e Expression) Lt(v any) Predicate { return compare("<", e, v) }

// Le compares less-than-or-equal.
func (e Expression) Le(v any) Predicate { return compare("<=", e, v) }

// Like matches against a pattern.
func (e Expression) Like(v any) Predicate { return compare("LIKE", e, v) }

// In tests membership: values lift as bound parameters, expressions render
// in place, and a single [Select] or [Compound] argument becomes
// IN (subquery). An empty list fails the statement.
func (e Expression) In(vals ...any) Predicate {
	if len(vals) == 1 {
		if q, ok := vals[0].(Query); ok {
			return Predicate{inNode{expr: e.node, sub: q}}
		}
	}
	items := make([]exprNode, len(vals))
	for i, v := range vals {
		items[i] = lift(v)
	}
	return Predicate{inNode{expr: e.node, items: items}}
}

// IsNull tests for NULL.
func (e Expression) IsNull() Predicate {
	return Predicate{
		nullNode{expr: e.node},
	}
}

// IsNotNull tests for NOT NULL.
func (e Expression) IsNotNull() Predicate {
	return Predicate{
		nullNode{expr: e.node, not: true},
	}
}

// And conjoins predicates, dropping empty operands; zero remaining operands
// yield the empty predicate and one passes through unwrapped.
func And(preds ...Predicate) Predicate {
	return combine("AND", preds)
}

// Or disjoins predicates with the same empty-operand rules as [And].
func Or(preds ...Predicate) Predicate {
	return combine("OR", preds)
}

// Not negates a predicate. Negating the empty predicate fails the statement.
func Not(p Predicate) Predicate {
	return Predicate{notNode{p.node}}
}

func compare(op string, e Expression, v any) Predicate {
	return Predicate{
		compareNode{op: op, left: e.node, right: lift(v)},
	}
}

func combine(op string, preds []Predicate) Predicate {
	nodes := make([]exprNode, 0, len(preds))
	for _, p := range preds {
		if p.node != nil {
			nodes = append(nodes, p.node)
		}
	}
	switch len(nodes) {
	case 0:
		return Predicate{}
	case 1:
		return Predicate{node: nodes[0]}
	default:
		return Predicate{node: logicNode{op: op, preds: nodes}}
	}
}
