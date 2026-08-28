package operation

import (
	"fmt"
	"slices"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
)

// Insertion is the identity-returning insert command: one row of values,
// with the engine handing back the caller-named identity and version fields
// it minted. It lowers through the write statements' Returning clause, so
// it requires a dialect implementing ast.ReturningRenderer and fails typed
// otherwise. Which columns exist, including what the version column is
// called, is the consumer's schema decision.
type Insertion struct {
	Into     string
	Values   []ast.Assignment
	Identity Field
	Version  Field
}

// SQL lowers the command to its statement: INSERT with one bound row,
// returning the identity and version aliased to their contract names — the
// scan order a runner reads them in.
func (i Insertion) SQL(d database.Dialect) (ast.SQL, error) {
	if i.Into == "" {
		return ast.SQL{}, fmt.Errorf("%w: insertion requires a table", ast.ErrInvalidStatement)
	}
	if len(i.Values) == 0 {
		return ast.SQL{}, fmt.Errorf("%w: insertion requires values", ast.ErrInvalidStatement)
	}
	if i.Identity.Name == "" || i.Identity.Expr.Empty() {
		return ast.SQL{}, fmt.Errorf("%w: insertion requires an identity field", ast.ErrInvalidStatement)
	}
	if i.Version.Name == "" || i.Version.Expr.Empty() {
		return ast.SQL{}, fmt.Errorf("%w: insertion requires a version field", ast.ErrInvalidStatement)
	}

	columns := make([]string, len(i.Values))
	row := make([]any, len(i.Values))
	for j, a := range i.Values {
		columns[j] = a.Column
		row[j] = a.Value
	}

	return ast.Insert{
		Table:   i.Into,
		Columns: columns,
		Rows:    [][]any{row},
		Returning: []ast.Column{
			{Expr: i.Identity.Expr, Alias: i.Identity.Name},
			{Expr: i.Version.Expr, Alias: i.Version.Name},
		},
	}.Render(d)
}

// Guard is the optimistic-concurrency spec: the consumer's version column —
// a schema decision the library never fixes — and the version the caller
// expects. Within a guarded command the contract manages that column
// exclusively: the guard appends the increment itself and rejects a caller
// assignment to it, which is what makes the new version deterministic.
type Guard struct {
	Column  string
	Version int64
}

// Guarded is a guarded command's envelope: the command statement, run
// first, and the check statement, run only when the command touched no row,
// splitting not-found from version-mismatch. Both are pure standard SQL;
// the runner in the exec package is the contract and this portable pair
// sits behind it.
type Guarded struct {
	Command ast.SQL
	Check   ast.SQL
}

// GuardedUpdate is the guarded update command: SET under the key and
// version guard, the version increment appended by the guard itself.
type GuardedUpdate struct {
	Table string
	Key   Field
	ID    any
	Guard Guard
	Set   []ast.Assignment
}

// SQL lowers the command to its [Guarded] pair.
func (u GuardedUpdate) SQL(d database.Dialect) (Guarded, error) {
	if len(u.Set) == 0 {
		return Guarded{}, fmt.Errorf("%w: guarded update requires a SET list", ast.ErrInvalidStatement)
	}
	for _, a := range u.Set {
		if a.Column == u.Guard.Column {
			return Guarded{}, fmt.Errorf("%w: the guard manages the %s column", ast.ErrInvalidStatement, u.Guard.Column)
		}
	}
	set := append(slices.Clone(u.Set), ast.Assignment{
		Column: u.Guard.Column,
		Value:  ast.Raw(u.Guard.Column + " + 1"),
	})
	return guarded(d, u.Table, u.Key, u.ID, u.Guard, set, false)
}

// GuardedDelete is the guarded delete command: the row goes only when both
// the key and the expected version match.
type GuardedDelete struct {
	Table string
	Key   Field
	ID    any
	Guard Guard
}

// SQL lowers the command to its [Guarded] pair.
func (del GuardedDelete) SQL(d database.Dialect) (Guarded, error) {
	return guarded(d, del.Table, del.Key, del.ID, del.Guard, nil, true)
}

// guarded lowers both guarded shapes: the command under the shared
// key-and-version WHERE, and the check selecting the current version by key
// alone.
func guarded(
	d database.Dialect,
	table string,
	key Field,
	id any,
	g Guard,
	set []ast.Assignment,
	deletion bool,
) (Guarded, error) {
	if table == "" {
		return Guarded{}, fmt.Errorf("%w: guarded command requires a table", ast.ErrInvalidStatement)
	}
	if key.Name == "" || key.Expr.Empty() {
		return Guarded{}, fmt.Errorf("%w: guarded command requires a key field", ast.ErrInvalidStatement)
	}
	if g.Column == "" {
		return Guarded{}, fmt.Errorf("%w: guarded command requires a version column", ast.ErrInvalidStatement)
	}

	where := ast.And(
		key.Expr.Eq(id),
		ast.Col(g.Column).Eq(g.Version),
	)

	var command ast.SQL
	var err error
	if deletion {
		command, err = ast.Delete{Table: table, Where: where}.Render(d)
	} else {
		command, err = ast.Update{Table: table, Set: set, Where: where}.Render(d)
	}
	if err != nil {
		return Guarded{}, err
	}

	check, err := ast.Select{
		Columns: []ast.Column{{Expr: ast.Col(g.Column)}},
		From:    ast.Table(table),
		Where:   key.Expr.Eq(id),
	}.Render(d)
	if err != nil {
		return Guarded{}, err
	}

	return Guarded{Command: command, Check: check}, nil
}
