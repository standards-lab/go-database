package ast

import "github.com/standards-lab/go-database"

// Delete is a delete statement as a value: the plain table name — joined or
// aliased DML targets are engine-divergent and stay out of the vocabulary —
// and the WHERE predicate. An absent Where deletes every row; guard against
// that at the operation layer, not here. Returning is the declared-native
// clause, rendered only through [ReturningRenderer].
type Delete struct {
	Table     string
	Where     Predicate
	Returning []Column
}

// Render renders the statement through the dialect, with the same contract
// as [Select.Render]; a Returning list on a dialect without
// [ReturningRenderer] fails with [UnsupportedFeatureError].
func (d Delete) Render(dialect database.Dialect) (SQL, error) {
	return renderSQL(dialect, d)
}

func (d Delete) render(r *renderer, mode queryMode) {
	if mode != modeTop {
		r.fail("a write statement belongs at the outermost position")
		return
	}
	if d.Table == "" {
		r.fail("delete missing its table")
		return
	}
	r.write("DELETE FROM " + d.Table)
	if d.Where.node != nil {
		r.write(" WHERE ")
		r.expr(d.Where.node)
	}
	renderReturning(r, d.Returning)
}
