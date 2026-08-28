package exec_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/exec"
)

func TestList(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{cols: []string{"count"}, rows: [][]driver.Value{{int64(7)}}},
		{cols: []string{"id", "name"}, rows: [][]driver.Value{
			{"a", "Ada"},
			{"b", "Bo"},
		}},
	}}
	db := newDB(t, conn)

	items, total, err := exec.List(context.Background(), db, personProjection(), pageDirectives(), scanPerson)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 7 {
		t.Errorf("total = %d, want 7", total)
	}
	if len(items) != 2 || items[0].Name != "Ada" || items[1].ID != "b" {
		t.Errorf("items = %+v, want Ada then b", items)
	}
	if len(conn.queriedSQL) != 2 ||
		!strings.HasPrefix(conn.queriedSQL[0], "SELECT COUNT(*)") ||
		!strings.Contains(conn.queriedSQL[1], "ORDER BY") {
		t.Errorf("queries = %q, want the count then the page", conn.queriedSQL)
	}
}

func TestListCountErrorMapped(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{err: errors.New("boom")},
	}}
	db := newDB(t, conn)

	_, _, err := exec.List(context.Background(), db, personProjection(), pageDirectives(), scanPerson)
	if !errors.Is(err, errMapped) {
		t.Errorf("List() error = %v, want it mapped through the dialect", err)
	}
}

func TestOne(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{cols: []string{"id", "name"}, rows: [][]driver.Value{{"a", "Ada"}}},
	}}
	db := newDB(t, conn)

	item, err := exec.One(context.Background(), db, personProjection(), "name", "Ada", scanPerson)
	if err != nil {
		t.Fatalf("One() error: %v", err)
	}
	if item.ID != "a" {
		t.Errorf("item = %+v, want id a", item)
	}
}

func TestOneNoRows(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{cols: []string{"id", "name"}},
	}}
	db := newDB(t, conn)

	_, err := exec.One(context.Background(), db, personProjection(), "name", "Nobody", scanPerson)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("One() error = %v, want sql.ErrNoRows", err)
	}
}

func TestOneUnknownFieldDoesNotQuery(t *testing.T) {
	conn := &fakeConn{}
	db := newDB(t, conn)

	_, err := exec.One(context.Background(), db, personProjection(), "nope", 1, scanPerson)
	if err == nil {
		t.Fatal("One() with an unknown field succeeded")
	}
	if len(conn.queriedSQL) != 0 {
		t.Errorf("queries = %q, want none before lowering fails", conn.queriedSQL)
	}
}

func TestQueryErrorMapped(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{err: errors.New("boom")},
	}}
	db := newDB(t, conn)

	stmt, err := personProjection().One(db.Dialect(), "id", "x")
	if err != nil {
		t.Fatalf("One() lowering error: %v", err)
	}
	if _, err := exec.Query(context.Background(), db, stmt, scanPerson); !errors.Is(err, errMapped) {
		t.Errorf("Query() error = %v, want it mapped through the dialect", err)
	}
}

func TestQueryRunsInsideTransaction(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{cols: []string{"id", "name"}, rows: [][]driver.Value{{"a", "Ada"}}},
	}}
	tx := newTx(t, conn)

	stmt, err := personProjection().One(tx.Dialect(), "id", "a")
	if err != nil {
		t.Fatalf("One() lowering error: %v", err)
	}
	items, err := exec.Query(context.Background(), tx, stmt, scanPerson)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("items = %+v, want one row through the transaction session", items)
	}
}
