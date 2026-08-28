package database_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/standards-lab/go-database"
)

var (
	_ database.Session = (*database.DB)(nil)
	_ database.Session = (*database.Tx)(nil)
)

// txConnector hands out one shared connection whose transaction handles
// record commits and rollbacks, so the Tx seam's behavior is observable.
type txConnector struct {
	conn *txConn
}

func (c *txConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }

func (c *txConnector) Driver() driver.Driver { return stubDriver{} }

type txConn struct {
	committed  int
	rolledBack int
	commitErr  error
}

func (*txConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (*txConn) Close() error { return nil }

func (c *txConn) Begin() (driver.Tx, error) { return txHandle{conn: c}, nil }

func (*txConn) Ping(context.Context) error { return nil }

type txHandle struct {
	conn *txConn
}

func (h txHandle) Commit() error {
	h.conn.committed++
	return h.conn.commitErr
}

func (h txHandle) Rollback() error {
	h.conn.rolledBack++
	return nil
}

// mappingDialect classifies every error by wrapping errMapped, proving where
// MapError is applied.
type mappingDialect struct{ stubDialect }

var errMapped = errors.New("mapped")

func (mappingDialect) MapError(err error) error {
	return fmt.Errorf("%w: %w", errMapped, err)
}

func startedTxDB(t *testing.T, conn *txConn, d database.Dialect) *database.DB {
	t.Helper()
	db := database.New(sql.OpenDB(&txConnector{conn: conn}), d, finalizedConfig(t))
	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Shutdown(context.Background()) })
	return db
}

func TestBeginBeforeStart(t *testing.T) {
	db := database.New(sql.OpenDB(&txConnector{conn: &txConn{}}), stubDialect{}, finalizedConfig(t))
	if _, err := db.Begin(context.Background()); !errors.Is(err, database.ErrNotReady) {
		t.Errorf("Begin() error = %v, want ErrNotReady", err)
	}
}

func TestTxCarriesDialect(t *testing.T) {
	db := startedTxDB(t, &txConn{}, stubDialect{})
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if tx.Dialect() != db.Dialect() {
		t.Error("Tx.Dialect() differs from DB.Dialect()")
	}
}

func TestExecTxCommits(t *testing.T) {
	conn := &txConn{}
	db := startedTxDB(t, conn, stubDialect{})

	if err := database.ExecTx(context.Background(), db, func(tx *database.Tx) error {
		return nil
	}); err != nil {
		t.Fatalf("ExecTx() error: %v", err)
	}
	if conn.committed != 1 || conn.rolledBack != 0 {
		t.Errorf("committed %d, rolled back %d; want 1 commit and no rollback", conn.committed, conn.rolledBack)
	}
}

func TestExecTxRollsBackOnError(t *testing.T) {
	conn := &txConn{}
	db := startedTxDB(t, conn, stubDialect{})

	unit := errors.New("unit failed")
	err := database.ExecTx(context.Background(), db, func(tx *database.Tx) error {
		return unit
	})
	if !errors.Is(err, unit) {
		t.Errorf("ExecTx() error = %v, want the unit's own error", err)
	}
	if conn.committed != 0 || conn.rolledBack != 1 {
		t.Errorf("committed %d, rolled back %d; want no commit and 1 rollback", conn.committed, conn.rolledBack)
	}
}

func TestCommitMapsThroughDialect(t *testing.T) {
	conn := &txConn{commitErr: errors.New("deferred constraint violation")}
	db := startedTxDB(t, conn, mappingDialect{})

	err := database.ExecTx(context.Background(), db, func(tx *database.Tx) error {
		return nil
	})
	if !errors.Is(err, errMapped) {
		t.Errorf("ExecTx() commit error = %v, want it mapped through the dialect", err)
	}
}
