package exec

import (
	"context"
	"database/sql"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/operation"
)

// Scan reads one row of a statement's result into a T. A domain package
// supplies one per entity, scanning in its projection's select-list order:
// key first, then each field in declaration order.
type Scan[T any] func(*sql.Rows) (T, error)

// List runs the list query: the projection's count and page statements over
// one shared WHERE, returning the page's items and the total row count.
func List[T any](
	ctx context.Context,
	s database.Session,
	p operation.Projection,
	dir operation.Directives,
	scan Scan[T],
) ([]T, int, error) {
	list, err := p.List(s.Dialect(), dir)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.
		QueryRowContext(ctx, list.Count.Text, list.Count.Args...).
		Scan(&total); err != nil {
		return nil, 0, s.Dialect().MapError(err)
	}

	items, err := Query(ctx, s, list.Page, scan)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// One runs the single-row query. A missing row is [sql.ErrNoRows]; an
// unknown field is the operation package's typed error, surfaced before
// anything executes; when the filtered field is not unique, the first row
// wins, as with [sql.DB.QueryRow].
func One[T any](
	ctx context.Context,
	s database.Session,
	p operation.Projection,
	field string,
	value any,
	scan Scan[T],
) (T, error) {
	var zero T
	stmt, err := p.One(s.Dialect(), field, value)
	if err != nil {
		return zero, err
	}
	items, err := Query(ctx, s, stmt, scan)
	if err != nil {
		return zero, err
	}
	if len(items) == 0 {
		return zero, sql.ErrNoRows
	}
	return items[0], nil
}

// Query executes one rendered statement and scans every row it returns,
// with a nil slice for zero rows.
func Query[T any](
	ctx context.Context,
	s database.Session,
	stmt ast.SQL,
	scan Scan[T],
) ([]T, error) {
	rows, err := s.QueryContext(ctx, stmt.Text, stmt.Args...)
	if err != nil {
		return nil, s.Dialect().MapError(err)
	}
	defer func() { _ = rows.Close() }()

	var items []T
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, s.Dialect().MapError(err)
	}
	return items, nil
}
