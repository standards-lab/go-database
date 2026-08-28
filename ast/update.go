package ast

import "github.com/standards-lab/go-database"

// Assignment is one SET entry: the target column and its value. The value
// lifts like any operand — an [Expression] renders in place, so
// Raw("version + 1") assigns a computation, and any other Go value binds as
// a parameter.
type Assignment struct {
	Column string
	Value  any
}

// Update is an update statement as a value: the plain table name — joined
// or aliased DML targets are engine-divergent and stay out of the
// vocabulary — the SET list, and the WHERE predicate. An absent Where
// updates every row; guard against that at the operation layer, not here.
// Returning is the declared-native clause, rendered only through
// [ReturningRenderer].
type Update struct {
	Table     string
	Set       []Assignment
	Where     Predicate
	Returning []Column
}

// Render renders the statement through the dialect, with the same contract
// as [Select.Render]; a Returning list on a dialect without
// [ReturningRenderer] fails with [UnsupportedFeatureError].
func (u Update) Render(d database.Dialect) (SQL, error) {
	return renderSQL(d, u)
}

func (u Update) render(r *renderer, mode queryMode) {
	if mode != modeTop {
		r.fail("a write statement belongs at the outermost position")
		return
	}
	if u.Table == "" {
		r.fail("update missing its table")
		return
	}
	if len(u.Set) == 0 {
		r.fail("update missing its SET list")
		return
	}
	r.write("UPDATE " + u.Table + " SET ")
	for j, a := range u.Set {
		if a.Column == "" {
			r.fail("assignment missing its column")
			return
		}
		if j > 0 {
			r.write(", ")
		}
		r.write(a.Column + " = ")
		r.expr(lift(a.Value))
	}
	if u.Where.node != nil {
		r.write(" WHERE ")
		r.expr(u.Where.node)
	}
	renderReturning(r, u.Returning)
}
