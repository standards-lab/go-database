package operation

import (
	"fmt"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
)

// Projection is a read model over one FROM: name-to-expression fields, one
// key field, and the CTEs the FROM references. It is plain data; every read
// against it goes through the query operations [Projection.List] and
// [Projection.One]. The key field is the identity a read pages by — it
// joins every ORDER BY as the tie-breaker, so offset paging is stable. CTEs
// declared here hoist to the top of every generated statement, the one
// placement every engine accepts.
type Projection struct {
	With   []ast.CTE
	From   ast.TableRef
	Key    Field
	Fields []Field
}

// List is the list query's envelope: the count and page statements over one
// shared WHERE clause. The consumer runs both — Count for the total, Page
// for the rows — and they can never disagree.
type List struct {
	Count ast.SQL
	Page  ast.SQL
}

// List forms the list query, lowering the directives onto the projection:
// field names resolve to expressions or fail with [UnknownFieldError],
// filters become the shared WHERE clause, sorts gain the key tie-breaker
// whenever the caller didn't sort by it, and the page directive becomes
// bound offset/fetch arguments.
func (p Projection) List(d database.Dialect, dir Directives) (List, error) {
	if dir.Page.Number < 1 {
		return List{}, fmt.Errorf("%w: page number must be at least 1", ast.ErrInvalidStatement)
	}
	if dir.Page.Size < 1 {
		return List{}, fmt.Errorf("%w: page size must be at least 1", ast.ErrInvalidStatement)
	}
	fields, err := p.fieldMap()
	if err != nil {
		return List{}, err
	}

	preds := make([]ast.Predicate, 0, len(dir.Filters)+1)
	for _, f := range dir.Filters {
		pred, err := lowerFilter(fields, f)
		if err != nil {
			return List{}, err
		}
		preds = append(preds, pred)
	}
	preds = append(preds, dir.Where)
	where := ast.And(preds...)

	orders := make([]ast.Order, 0, len(dir.Sort)+1)
	keySorted := false
	for _, s := range dir.Sort {
		expr, ok := fields[s.Field]
		if !ok {
			return List{}, &UnknownFieldError{
				Field: s.Field,
				Use:   FieldUseSort,
			}
		}
		if s.Field == p.Key.Name {
			keySorted = true
		}
		orders = append(orders, ast.Order{Expr: expr, Descending: s.Descending})
	}
	if !keySorted {
		orders = append(orders, ast.Order{Expr: p.Key.Expr})
	}

	page, err := ast.Select{
		With:    p.With,
		Columns: p.selectList(),
		From:    p.From,
		Where:   where,
		OrderBy: orders,
		Paging: &ast.Paging{
			Offset: (dir.Page.Number - 1) * dir.Page.Size,
			Fetch:  dir.Page.Size,
		},
	}.Render(d)
	if err != nil {
		return List{}, err
	}

	count, err := ast.Select{
		With:    p.With,
		Columns: []ast.Column{{Expr: ast.Fn("COUNT", ast.Raw("*"))}},
		From:    p.From,
		Where:   where,
	}.Render(d)
	if err != nil {
		return List{}, err
	}

	return List{Count: count, Page: page}, nil
}

// One forms the single-row query: the full projection under one equality
// filter on a projected field, with no ordering, no paging, and no count.
// An unknown field is an [UnknownFieldError]. When the filtered field is
// not unique, the first row wins; execution outcomes, sql.ErrNoRows
// included, belong to the runner.
func (p Projection) One(d database.Dialect, field string, value any) (ast.SQL, error) {
	fields, err := p.fieldMap()
	if err != nil {
		return ast.SQL{}, err
	}
	expr, ok := fields[field]
	if !ok {
		return ast.SQL{}, &UnknownFieldError{Field: field, Use: FieldUseFilter}
	}
	return ast.Select{
		With:    p.With,
		Columns: p.selectList(),
		From:    p.From,
		Where:   expr.Eq(value),
	}.Render(d)
}

// selectList assembles the projection's select list: key first, every field
// aliased to its contract name — the scan order every query shares.
func (p Projection) selectList() []ast.Column {
	columns := make([]ast.Column, 0, len(p.Fields)+1)
	columns = append(columns, ast.Column{Expr: p.Key.Expr, Alias: p.Key.Name})
	for _, f := range p.Fields {
		columns = append(columns, ast.Column{Expr: f.Expr, Alias: f.Name})
	}
	return columns
}

// fieldMap validates the projection and indexes its fields by contract
// name, key included.
func (p Projection) fieldMap() (map[string]ast.Expression, error) {
	if p.Key.Name == "" || p.Key.Expr.Empty() {
		return nil, fmt.Errorf("%w: projection requires a key field", ast.ErrInvalidStatement)
	}
	fields := map[string]ast.Expression{p.Key.Name: p.Key.Expr}
	for _, f := range p.Fields {
		if f.Name == "" || f.Expr.Empty() {
			return nil, fmt.Errorf("%w: projection field missing a name or expression", ast.ErrInvalidStatement)
		}
		if _, ok := fields[f.Name]; ok {
			return nil, fmt.Errorf("%w: duplicate projection field: %q", ast.ErrInvalidStatement, f.Name)
		}
		fields[f.Name] = f.Expr
	}
	return fields, nil
}
