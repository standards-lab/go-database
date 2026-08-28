package exec_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/operation"
)

// fakeConn scripts driver-level responses in call order and records every
// statement it receives, so the runners are exercised hermetically.
type fakeConn struct {
	queries []queryScript
	execs   []execScript

	queriedSQL []string
	execedSQL  []string
}

type queryScript struct {
	cols []string
	rows [][]driver.Value
	err  error
}

type execScript struct {
	affected int64
	err      error
}

type fakeConnector struct {
	conn *fakeConn
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }

func (c *fakeConnector) Driver() driver.Driver { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("open by DSN unsupported")
}

func (*fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (*fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) { return fakeTx{}, nil }

func (*fakeConn) Ping(context.Context) error { return nil }

func (c *fakeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.queriedSQL = append(c.queriedSQL, query)
	if len(c.queries) == 0 {
		return nil, errors.New("unscripted query")
	}
	s := c.queries[0]
	c.queries = c.queries[1:]
	if s.err != nil {
		return nil, s.err
	}
	return &fakeRows{cols: s.cols, rows: s.rows}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.execedSQL = append(c.execedSQL, query)
	if len(c.execs) == 0 {
		return nil, errors.New("unscripted exec")
	}
	s := c.execs[0]
	c.execs = c.execs[1:]
	if s.err != nil {
		return nil, s.err
	}
	return driver.RowsAffected(s.affected), nil
}

type fakeTx struct{}

func (fakeTx) Commit() error { return nil }

func (fakeTx) Rollback() error { return nil }

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *fakeRows) Columns() []string { return r.cols }

func (r *fakeRows) Close() error { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// mapDialect classifies every driver error by wrapping errMapped, proving
// the runners route errors through MapError.
type mapDialect struct{}

var errMapped = errors.New("mapped")

func (mapDialect) Name() string { return "fake" }

func (mapDialect) Placeholder(n int) string { return "$" + strconv.Itoa(n) }

func (mapDialect) MapError(err error) error {
	return fmt.Errorf("%w: %w", errMapped, err)
}

func (mapDialect) RenderReturning(w ast.Writer, cols []ast.Column) {
	w.SQL("RETURNING ")
	for i, c := range cols {
		if i > 0 {
			w.SQL(", ")
		}
		w.Column(c)
	}
}

func newDB(t *testing.T, conn *fakeConn) *database.DB {
	t.Helper()
	cfg := database.Config{Name: "app"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}
	db := database.New(sql.OpenDB(&fakeConnector{conn: conn}), mapDialect{}, cfg)
	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Shutdown(context.Background()) })
	return db
}

func newTx(t *testing.T, conn *fakeConn) *database.Tx {
	t.Helper()
	tx, err := newDB(t, conn).Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

type person struct {
	ID   string
	Name string
}

func scanPerson(rows *sql.Rows) (person, error) {
	var p person
	err := rows.Scan(&p.ID, &p.Name)
	return p, err
}

func personProjection() operation.Projection {
	return operation.Projection{
		From: ast.Table("t"),
		Key:  operation.Field{Name: "id", Expr: ast.Col("t.id")},
		Fields: []operation.Field{
			{Name: "name", Expr: ast.Col("t.name")},
		},
	}
}

func pageDirectives() operation.Directives {
	return operation.Directives{Page: operation.Page{Number: 1, Size: 10}}
}
