package query

import (
	"fmt"

	"github.com/standards-lab/go-database"
)

// Statement is one rendered statement: the SQL text and its bound arguments
// in placeholder order.
type Statement struct {
	SQL  string
	Args []any
}

// Statements is a read's pair of statements over one WHERE clause. The
// consumer runs both: Count for the total, Page for the rows.
type Statements struct {
	Count Statement
	Page  Statement
}

// Statements lowers one read's directives onto the projection: field names
// resolve to expressions or fail with [UnknownFieldError], filters become
// the shared WHERE clause, sorts gain the key tie-breaker whenever the
// caller didn't sort by it, and the page directive becomes bound
// offset/fetch arguments. The returned pair renders through the dialect and
// shares one WHERE, so the count can never disagree with the page.
func (p Projection) Statements(d database.Dialect, dir Directives) (Statements, error) {
	fields, err := p.fieldMap()
	if err != nil {
		return Statements{}, err
	}
	if dir.Page.Number < 1 {
		return Statements{}, fmt.Errorf("%w: page number must be at least 1", ErrInvalidStatement)
	}
	if dir.Page.Size < 1 {
		return Statements{}, fmt.Errorf("%w: page size must be at least 1", ErrInvalidStatement)
	}

	preds := make([]Predicate, 0, len(dir.Filters)+1)
	for _, f := range dir.Filters {
		pred, err := lowerFilter(fields, f)
		if err != nil {
			return Statements{}, err
		}
		preds = append(preds, pred)
	}
	preds = append(preds, dir.Where)
	where := And(preds...)

	orders := make([]Order, 0, len(dir.Sort)+1)
	keySorted := false
	for _, s := range dir.Sort {
		expr, ok := fields[s.Field]
		if !ok {
			return Statements{}, &UnknownFieldError{
				Field: s.Field,
				Use:   FieldUseSort,
			}
		}
		if s.Field == p.Key.Name {
			keySorted = true
		}
		orders = append(orders, Order{Expr: expr, Descending: s.Descending})
	}
	if !keySorted {
		orders = append(orders, Order{Expr: p.Key.Expr})
	}

	columns := make([]Column, 0, len(p.Fields)+1)
	columns = append(columns, Column{Expr: p.Key.Expr, Alias: p.Key.Name})
	for _, f := range p.Fields {
		columns = append(columns, Column{Expr: f.Expr, Alias: f.Name})
	}

	pageSQL, pageArgs, err := Select{
		With:    p.With,
		Columns: columns,
		From:    p.From,
		Where:   where,
		OrderBy: orders,
		Paging: &Paging{
			Offset: (dir.Page.Number - 1) * dir.Page.Size,
			Fetch:  dir.Page.Size,
		},
	}.SQL(d)
	if err != nil {
		return Statements{}, err
	}

	countSQL, countArgs, err := Select{
		With:    p.With,
		Columns: []Column{{Expr: Fn("COUNT", Raw("*"))}},
		From:    p.From,
		Where:   where,
	}.SQL(d)
	if err != nil {
		return Statements{}, err
	}

	return Statements{
		Count: Statement{SQL: countSQL, Args: countArgs},
		Page:  Statement{SQL: pageSQL, Args: pageArgs},
	}, nil
}

// fieldMap validates the projection and indexes its fields by contract name.
func (p Projection) fieldMap() (map[string]Expression, error) {
	if p.Key.Name == "" || p.Key.Expr.node == nil {
		return nil, fmt.Errorf("%w: projection requires a key field", ErrInvalidStatement)
	}
	fields := map[string]Expression{p.Key.Name: p.Key.Expr}
	for _, f := range p.Fields {
		if f.Name == "" || f.Expr.node == nil {
			return nil, fmt.Errorf("%w: projection field missing a name or expression", ErrInvalidStatement)
		}
		if _, ok := fields[f.Name]; ok {
			return nil, fmt.Errorf("%w: duplicate projection field: %q", ErrInvalidStatement, f.Name)
		}
		fields[f.Name] = f.Expr
	}
	return fields, nil
}

// lowerFilter resolves one filter directive against the field contract and
// lowers it to a predicate.
func lowerFilter(fields map[string]Expression, f Filter) (Predicate, error) {
	expr, ok := fields[f.Field]
	if !ok {
		return Predicate{}, &UnknownFieldError{Field: f.Field, Use: FieldUseFilter}
	}
	switch f.Op {
	case OpEq:
		return expr.Eq(f.Value), nil
	case OpNe:
		return expr.Ne(f.Value), nil
	case OpGt:
		return expr.Gt(f.Value), nil
	case OpGe:
		return expr.Ge(f.Value), nil
	case OpLt:
		return expr.Lt(f.Value), nil
	case OpLe:
		return expr.Le(f.Value), nil
	case OpLike:
		return expr.Like(f.Value), nil
	case OpIsNull:
		return expr.IsNull(), nil
	case OpIsNotNull:
		return expr.IsNotNull(), nil
	case OpIn:
		vals, ok := f.Value.([]any)
		if !ok {
			return Predicate{}, fmt.Errorf("%w: in filter on %q requires a []any value", ErrInvalidStatement, f.Field)
		}
		return expr.In(vals...), nil
	default:
		return Predicate{}, &UnknownOperatorError{Op: f.Op}
	}
}
