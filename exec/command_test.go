package exec_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/ast"
	"github.com/standards-lab/go-database/exec"
	"github.com/standards-lab/go-database/operation"
)

func testInsertion() operation.Insertion {
	return operation.Insertion{
		Into: "organization",
		Values: []ast.Assignment{
			{Column: "code", Value: "acme"},
			{Column: "name", Value: "Acme"},
		},
		Identity: operation.Field{Name: "id", Expr: ast.Col("id")},
		Version:  operation.Field{Name: "version", Expr: ast.Col("version")},
	}
}

func testGuardedUpdate() operation.GuardedUpdate {
	return operation.GuardedUpdate{
		Table: "organization",
		Key:   operation.Field{Name: "id", Expr: ast.Col("id")},
		ID:    "x",
		Guard: operation.Guard{Column: "version", Version: 3},
		Set:   []ast.Assignment{{Column: "name", Value: "Acme"}},
	}
}

func testGuardedDelete() operation.GuardedDelete {
	return operation.GuardedDelete{
		Table: "organization",
		Key:   operation.Field{Name: "id", Expr: ast.Col("id")},
		ID:    "x",
		Guard: operation.Guard{Column: "version", Version: 3},
	}
}

func TestInsert(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{cols: []string{"id", "version"}, rows: [][]driver.Value{{"id-1", int64(1)}}},
	}}
	tx := newTx(t, conn)

	id, err := exec.Insert(context.Background(), tx, testInsertion())
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}
	if id.ID != "id-1" || id.Version != 1 {
		t.Errorf("identity = %+v, want id-1 at version 1", id)
	}
	if len(conn.queriedSQL) != 1 || !strings.Contains(conn.queriedSQL[0], "RETURNING id AS id, version AS version") {
		t.Errorf("queries = %q, want the returning insert", conn.queriedSQL)
	}
}

func TestInsertErrorMapped(t *testing.T) {
	conn := &fakeConn{queries: []queryScript{
		{err: errors.New("unique violation")},
	}}
	tx := newTx(t, conn)

	if _, err := exec.Insert(context.Background(), tx, testInsertion()); !errors.Is(err, errMapped) {
		t.Errorf("Insert() error = %v, want it mapped through the dialect", err)
	}
}

func TestUpdateSuccess(t *testing.T) {
	conn := &fakeConn{execs: []execScript{{affected: 1}}}
	tx := newTx(t, conn)

	version, err := exec.Update(context.Background(), tx, testGuardedUpdate())
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if version != 4 {
		t.Errorf("version = %d, want the deterministic increment to 4", version)
	}
	if len(conn.queriedSQL) != 0 {
		t.Errorf("queries = %q, want no check on the happy path", conn.queriedSQL)
	}
}

func TestUpdateVersionMismatch(t *testing.T) {
	conn := &fakeConn{
		execs:   []execScript{{affected: 0}},
		queries: []queryScript{{cols: []string{"version"}, rows: [][]driver.Value{{int64(5)}}}},
	}
	tx := newTx(t, conn)

	_, err := exec.Update(context.Background(), tx, testGuardedUpdate())
	if !errors.Is(err, database.ErrVersionMismatch) {
		t.Fatalf("Update() error = %v, want ErrVersionMismatch", err)
	}
	if !strings.Contains(err.Error(), "expected 3") || !strings.Contains(err.Error(), "current 5") {
		t.Errorf("error = %v, want both versions carried for diagnostics", err)
	}
}

func TestUpdateNotFound(t *testing.T) {
	conn := &fakeConn{
		execs:   []execScript{{affected: 0}},
		queries: []queryScript{{cols: []string{"version"}}},
	}
	tx := newTx(t, conn)

	_, err := exec.Update(context.Background(), tx, testGuardedUpdate())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Update() error = %v, want sql.ErrNoRows", err)
	}
}

func TestUpdateCommandErrorMapped(t *testing.T) {
	conn := &fakeConn{execs: []execScript{{err: errors.New("boom")}}}
	tx := newTx(t, conn)

	if _, err := exec.Update(context.Background(), tx, testGuardedUpdate()); !errors.Is(err, errMapped) {
		t.Errorf("Update() error = %v, want it mapped through the dialect", err)
	}
}

func TestDeleteSuccess(t *testing.T) {
	conn := &fakeConn{execs: []execScript{{affected: 1}}}
	tx := newTx(t, conn)

	if err := exec.Delete(context.Background(), tx, testGuardedDelete()); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
}

func TestDeleteVersionMismatch(t *testing.T) {
	conn := &fakeConn{
		execs:   []execScript{{affected: 0}},
		queries: []queryScript{{cols: []string{"version"}, rows: [][]driver.Value{{int64(9)}}}},
	}
	tx := newTx(t, conn)

	err := exec.Delete(context.Background(), tx, testGuardedDelete())
	if !errors.Is(err, database.ErrVersionMismatch) {
		t.Errorf("Delete() error = %v, want ErrVersionMismatch", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	conn := &fakeConn{
		execs:   []execScript{{affected: 0}},
		queries: []queryScript{{cols: []string{"version"}}},
	}
	tx := newTx(t, conn)

	err := exec.Delete(context.Background(), tx, testGuardedDelete())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Delete() error = %v, want sql.ErrNoRows", err)
	}
}
