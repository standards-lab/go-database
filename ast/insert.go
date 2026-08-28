package ast

import "github.com/standards-lab/go-database"

// Insert is an insert statement as a value: the plain table name, an
// optional column list, and either literal Rows or a Query
// (INSERT ... SELECT), never both. Row values lift like operands —
// expressions render in place, plain values bind — and each row's width
// must match the column list when one is given. Returning is the
// declared-native clause, rendered only through [ReturningRenderer].
type Insert struct {
	Table     string
	Columns   []string
	Rows      [][]any
	Query     Query
	Returning []Column
}

// Render renders the statement through the dialect, with the same contract
// as [Select.Render]; a Returning list on a dialect without
// [ReturningRenderer] fails with [UnsupportedFeatureError].
func (i Insert) Render(d database.Dialect) (SQL, error) {
	return renderSQL(d, i)
}

func (i Insert) render(r *renderer, mode queryMode) {
	if mode != modeTop {
		r.fail("a write statement belongs at the outermost position")
		return
	}
	if i.Table == "" {
		r.fail("insert missing its table")
		return
	}
	if len(i.Rows) > 0 && i.Query != nil {
		r.fail("insert takes rows or a query, not both")
		return
	}
	if len(i.Rows) == 0 && i.Query == nil {
		r.fail("insert missing rows or a query")
		return
	}
	r.write("INSERT INTO " + i.Table)
	if len(i.Columns) > 0 {
		r.write(" (")
		for j, c := range i.Columns {
			if c == "" {
				r.fail("empty insert column")
				return
			}
			if j > 0 {
				r.write(", ")
			}
			r.write(c)
		}
		r.write(")")
	}
	switch {
	case i.Query != nil:
		r.write(" ")
		i.Query.render(r, modeSub)
	default:
		r.write(" VALUES ")
		for j, row := range i.Rows {
			if len(row) == 0 {
				r.fail("empty insert row")
				return
			}
			if len(i.Columns) > 0 && len(row) != len(i.Columns) {
				r.fail("insert row width does not match its column list")
				return
			}
			if j > 0 {
				r.write(", ")
			}
			r.write("(")
			for k, v := range row {
				if k > 0 {
					r.write(", ")
				}
				r.expr(lift(v))
			}
			r.write(")")
		}
	}
	renderReturning(r, i.Returning)
}
