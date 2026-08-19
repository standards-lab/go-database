package seed_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/standards-lab/go-database/seed"
)

// fakeConn is a minimal database/sql driver connection that counts
// transactions. The runner never prepares statements itself — SQL belongs to
// the load functions — so only Begin, Commit, and Rollback carry behavior.
type fakeConn struct {
	begun     int
	commits   int
	rollbacks int
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected Prepare") }
func (c *fakeConn) Close() error                        { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	c.begun++
	return &fakeTx{conn: c}, nil
}

type fakeTx struct{ conn *fakeConn }

func (t *fakeTx) Commit() error   { t.conn.commits++; return nil }
func (t *fakeTx) Rollback() error { t.conn.rollbacks++; return nil }

type fakeConnector struct{ conn *fakeConn }

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c *fakeConnector) Driver() driver.Driver                        { return nil }

// newFakeDB returns a *sql.DB whose every connection is the returned
// fakeConn, so transaction counts aggregate across the pool.
func newFakeDB() (*sql.DB, *fakeConn) {
	conn := &fakeConn{}
	return sql.OpenDB(&fakeConnector{conn: conn}), conn
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestNew_RequiresDBLoggerAndFormat(t *testing.T) {
	db, _ := newFakeDB()

	if _, err := seed.New(nil, discardLogger(), seed.JSON{}); err == nil {
		t.Error("New(nil db) = nil, want error")
	}
	if _, err := seed.New(db, nil, seed.JSON{}); err == nil {
		t.Error("New(nil logger) = nil, want error")
	}
	if _, err := seed.New(db, discardLogger()); err == nil {
		t.Error("New(no formats) = nil, want error")
	}
}

func TestNew_RejectsDuplicateExtension(t *testing.T) {
	db, _ := newFakeDB()

	_, err := seed.New(db, discardLogger(), seed.JSON{}, seed.JSON{})

	if err == nil {
		t.Fatal("New(JSON, JSON) = nil, want duplicate-extension error")
	}
	if !strings.Contains(err.Error(), ".json") {
		t.Errorf("New() error = %v, want it to name the extension", err)
	}
}

func TestRun_DecodesAndLoadsInsideOneTransaction(t *testing.T) {
	db, conn := newFakeDB()
	runner, err := seed.New(db, discardLogger(), seed.JSON{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type row struct {
		Code string `json:"code"`
	}
	data := fstest.MapFS{
		"seeds/rows.json": {Data: []byte(`[{"code":"hq"},{"code":"ops"}]`)},
	}

	var got []row
	step := seed.Table("seeds/rows.json", func(_ context.Context, tx *sql.Tx, rows []row) error {
		if tx == nil {
			t.Error("load received nil tx")
		}
		got = rows
		return nil
	})

	if err := runner.Run(context.Background(), data, step); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(got) != 2 || got[0].Code != "hq" || got[1].Code != "ops" {
		t.Errorf("loaded rows = %+v, want hq then ops", got)
	}
	if conn.begun != 1 || conn.commits != 1 || conn.rollbacks != 0 {
		t.Errorf("tx counts begun/commit/rollback = %d/%d/%d, want 1/1/0",
			conn.begun, conn.commits, conn.rollbacks)
	}
}

func TestRun_ExecutesStepsInOrderAndLogsEach(t *testing.T) {
	db, conn := newFakeDB()

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	runner, err := seed.New(db, logger, seed.JSON{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type row struct {
		Code string `json:"code"`
	}
	data := fstest.MapFS{
		"seeds/first.json":  {Data: []byte(`[{"code":"a"}]`)},
		"seeds/second.json": {Data: []byte(`[{"code":"b"}]`)},
	}

	var order []string
	loadInto := func(name string) func(context.Context, *sql.Tx, []row) error {
		return func(context.Context, *sql.Tx, []row) error {
			order = append(order, name)
			return nil
		}
	}

	err = runner.Run(context.Background(), data,
		seed.Table("seeds/first.json", loadInto("first")),
		seed.Table("seeds/second.json", loadInto("second")),
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("load order = %v, want [first second]", order)
	}
	if conn.commits != 2 {
		t.Errorf("commits = %d, want 2 (one transaction per step)", conn.commits)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, "seeds/first.json") || !strings.Contains(logged, "seeds/second.json") {
		t.Errorf("log output %q, want both step paths reported", logged)
	}
}

func TestRun_LoadErrorRollsBackAndNamesStep(t *testing.T) {
	db, conn := newFakeDB()
	runner, err := seed.New(db, discardLogger(), seed.JSON{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type row struct {
		Code string `json:"code"`
	}
	data := fstest.MapFS{
		"seeds/rows.json": {Data: []byte(`[{"code":"hq"}]`)},
	}
	sentinel := errors.New("constraint violated")
	step := seed.Table("seeds/rows.json", func(context.Context, *sql.Tx, []row) error {
		return sentinel
	})

	err = runner.Run(context.Background(), data, step)

	if !errors.Is(err, sentinel) {
		t.Fatalf("Run() error = %v, want the load error preserved", err)
	}
	if !strings.Contains(err.Error(), "seeds/rows.json") {
		t.Errorf("Run() error = %v, want the step path named", err)
	}
	if conn.commits != 0 || conn.rollbacks != 1 {
		t.Errorf("commit/rollback = %d/%d, want 0/1", conn.commits, conn.rollbacks)
	}
}

func TestRun_DecodeErrorRollsBack(t *testing.T) {
	db, conn := newFakeDB()
	runner, err := seed.New(db, discardLogger(), seed.JSON{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type row struct {
		Code string `json:"code"`
	}
	data := fstest.MapFS{
		"seeds/rows.json": {Data: []byte(`not json`)},
	}
	step := seed.Table("seeds/rows.json", func(context.Context, *sql.Tx, []row) error {
		t.Error("load ran despite a decode failure")
		return nil
	})

	err = runner.Run(context.Background(), data, step)

	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("Run() error = %v, want decode error", err)
	}
	if conn.commits != 0 || conn.rollbacks != 1 {
		t.Errorf("commit/rollback = %d/%d, want 0/1", conn.commits, conn.rollbacks)
	}
}

func TestRun_UnknownExtensionFailsBeforeAnyTransaction(t *testing.T) {
	db, conn := newFakeDB()
	runner, err := seed.New(db, discardLogger(), seed.JSON{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type row struct{}
	step := seed.Table("seeds/rows.yaml", func(context.Context, *sql.Tx, []row) error {
		return nil
	})

	err = runner.Run(context.Background(), fstest.MapFS{}, step)

	if err == nil || !strings.Contains(err.Error(), ".yaml") {
		t.Fatalf("Run() error = %v, want unknown-extension error naming .yaml", err)
	}
	if conn.begun != 0 {
		t.Errorf("begun = %d, want 0 (no transaction for an unloadable step)", conn.begun)
	}
}

func TestRun_MissingFileFailsBeforeAnyTransaction(t *testing.T) {
	db, conn := newFakeDB()
	runner, err := seed.New(db, discardLogger(), seed.JSON{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type row struct{}
	step := seed.Table("seeds/absent.json", func(context.Context, *sql.Tx, []row) error {
		return nil
	})

	err = runner.Run(context.Background(), fstest.MapFS{}, step)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Run() error = %v, want fs.ErrNotExist", err)
	}
	if conn.begun != 0 {
		t.Errorf("begun = %d, want 0 (no transaction for a missing file)", conn.begun)
	}
}
