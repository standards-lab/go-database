package database_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/standards-lab/go-database"
)

// stubConnector produces connections that consult its failure flag on every
// ping, so connectivity can be broken and restored mid-test — including for
// connections the pool has cached.
type stubConnector struct {
	fail atomic.Bool
}

func (c *stubConnector) Connect(context.Context) (driver.Conn, error) {
	if c.fail.Load() {
		return nil, errors.New("dial refused")
	}
	return stubConn{connector: c}, nil
}

func (c *stubConnector) Driver() driver.Driver { return stubDriver{} }

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("open by DSN unsupported")
}

type stubConn struct {
	connector *stubConnector
}

func (stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (stubConn) Close() error { return nil }

func (stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin unsupported")
}

func (s stubConn) Ping(context.Context) error {
	if s.connector.fail.Load() {
		return errors.New("ping refused")
	}
	return nil
}

func finalizedConfig(t *testing.T) database.Config {
	t.Helper()
	cfg := database.Config{Name: "app"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}
	return cfg
}

func newTestDB(t *testing.T, connector *stubConnector) *database.DB {
	t.Helper()
	db := database.New(sql.OpenDB(connector), finalizedConfig(t))
	t.Cleanup(func() { _ = db.Conn().Close() })
	return db
}

func wantPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("no panic, want one containing %q", want)
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, want) {
			t.Errorf("panic = %v, want it to contain %q", r, want)
		}
	}()
	fn()
}

func TestNew_PanicsOnUnfinalizedConfig(t *testing.T) {
	conn := sql.OpenDB(&stubConnector{})
	t.Cleanup(func() { _ = conn.Close() })

	wantPanic(t, "Config not finalized", func() {
		database.New(conn, database.Config{Name: "app"})
	})
}

func TestNew_PanicsOnNilConn(t *testing.T) {
	wantPanic(t, "nil conn", func() {
		database.New(nil, finalizedConfig(t))
	})
}

func TestNew_AppliesPoolSettings(t *testing.T) {
	cfg := database.Config{Name: "app", MaxOpenConns: new(42)}
	if err := cfg.Finalize(""); err != nil {
		t.Fatalf("finalize config: %v", err)
	}

	db := database.New(sql.OpenDB(&stubConnector{}), cfg)
	t.Cleanup(func() { _ = db.Conn().Close() })

	if got := db.Conn().Stats().MaxOpenConnections; got != 42 {
		t.Errorf("MaxOpenConnections = %d, want 42", got)
	}
}

func TestDB_Accessors(t *testing.T) {
	db := newTestDB(t, &stubConnector{})

	if db.Conn() == nil {
		t.Error("Conn() = nil")
	}
}

func TestDB_StartPingsAndReports(t *testing.T) {
	connector := &stubConnector{}
	db := newTestDB(t, connector)

	if db.Ready() {
		t.Error("Ready() = true before Start, want false")
	}
	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !db.Ready() {
		t.Error("Ready() = false after a successful Start, want true")
	}
}

func TestDB_StartFailureWrapsSentinelAndCause(t *testing.T) {
	connector := &stubConnector{}
	connector.fail.Store(true)
	db := newTestDB(t, connector)

	err := db.Start(context.Background())
	if err == nil {
		t.Fatal("Start succeeded against a refusing connector")
	}
	if !errors.Is(err, database.ErrConnectionFailed) {
		t.Errorf("errors.Is(err, ErrConnectionFailed) = false for %v", err)
	}
	// The dual wrap keeps the driver's cause legible.
	if !strings.Contains(err.Error(), "dial refused") {
		t.Errorf("error = %v, want it to carry the driver cause", err)
	}
	if db.Ready() {
		t.Error("Ready() = true after a failed Start, want false")
	}
}

func TestDB_ReadySelfHeals(t *testing.T) {
	connector := &stubConnector{}
	db := newTestDB(t, connector)

	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	connector.fail.Store(true)
	if db.Ready() {
		t.Error("Ready() = true during an outage, want false")
	}

	connector.fail.Store(false)
	if !db.Ready() {
		t.Error("Ready() = false after the outage cleared, want true")
	}
}

func TestDB_Ping(t *testing.T) {
	connector := &stubConnector{}
	db := newTestDB(t, connector)

	if err := db.Ping(context.Background()); !errors.Is(err, database.ErrNotReady) {
		t.Errorf("Ping before Start = %v, want ErrNotReady", err)
	}

	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("Ping = %v, want nil", err)
	}

	connector.fail.Store(true)
	if err := db.Ping(context.Background()); !errors.Is(err, database.ErrConnectionFailed) {
		t.Errorf("Ping during an outage = %v, want ErrConnectionFailed", err)
	}
}

func TestDB_ShutdownBeforeStart(t *testing.T) {
	db := newTestDB(t, &stubConnector{})

	// Closing a lazy pool that never connected is a clean no-op, so the
	// drain-after-failed-startup path cannot compound the failure.
	if err := db.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown before Start = %v, want nil", err)
	}
}

func TestDB_ShutdownClearsReadiness(t *testing.T) {
	db := newTestDB(t, &stubConnector{})

	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := db.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if db.Ready() {
		t.Error("Ready() = true after Shutdown, want false")
	}
	if err := db.Ping(context.Background()); !errors.Is(err, database.ErrNotReady) {
		t.Errorf("Ping after Shutdown = %v, want ErrNotReady", err)
	}
}
