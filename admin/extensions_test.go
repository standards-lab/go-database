package admin_test

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/admin"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/sqltest"
)

type extensionDialect struct{ testDialect }

func (extensionDialect) ExtensionState(param string) string {
	return "SELECT installed, default, path FROM extensions WHERE name = " + param
}

func (extensionDialect) ActivateExtension(name string) []string {
	return []string{`CREATE EXTENSION IF NOT EXISTS "` + name + `"`, `ALTER EXTENSION "` + name + `" UPDATE`}
}

func (extensionDialect) RemoveExtension(name string) string {
	return `DROP EXTENSION IF EXISTS "` + name + `" RESTRICT`
}

var _ migrate.ExtensionCatalog = extensionDialect{}

func extState(installed, def driver.Value, path bool) sqltest.Response {
	return sqltest.Response{Columns: []string{"installed", "default", "path"}, Rows: [][]driver.Value{{installed, def, path}}}
}

var (
	extCurrent     = extState("0.8.0", "0.8.0", true)
	extMissing     = extState(nil, "0.8.0", false)
	extUnavailable = extState(nil, nil, false)
	extNoPath      = extState("0.5.0", "0.8.0", false)
)

func newExtensionService(t *testing.T, declared []migrate.Extension, opts admin.Options, responses ...sqltest.Response) fixture {
	t.Helper()
	return newMigratorFixture(t, []migrate.Set{{Name: "app", Migrations: set}}, extensionDialect{},
		migrate.Options{Extensions: declared}, opts, responses...)
}

var vector = []migrate.Extension{{Name: "vector"}}

func TestExtensions_StatusListsNoneWhenUndeclared(t *testing.T) {
	s, _ := newService(t, current()...)
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Extensions == nil || len(st.Extensions) != 0 || !st.Ready {
		t.Fatalf("status = %+v", st)
	}
	out, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"extensions":[]`) {
		t.Errorf("json = %s", out)
	}
}

func TestExtensions_StatusReportsEachDeclarationInOrder(t *testing.T) {
	declared := []migrate.Extension{{Name: "vector"}, {Name: "hstore"}, {Name: "old", Removed: true}}
	f := newExtensionService(t, declared, admin.Options{},
		slices.Concat(current(), []sqltest.Response{extCurrent, extMissing, extState("1.0", "1.0", true)})...)
	st, err := f.service.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := []admin.ExtensionStatus{
		{Name: "vector", Installed: "0.8.0", Default: "0.8.0"},
		{Name: "hstore", Default: "0.8.0", Pending: true},
		{Name: "old", Removed: true, Installed: "1.0", Default: "1.0", Pending: true},
	}
	if !slices.Equal(st.Extensions, want) {
		t.Errorf("extensions = %+v, want %+v", st.Extensions, want)
	}
	out, err := json.Marshal(st.Extensions[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != `{"name":"vector","removed":false,"installed":"0.8.0","default":"0.8.0","pending":false}` {
		t.Errorf("json = %s", got)
	}
	if f.rec.Pending() != 0 {
		t.Errorf("pending responses = %d", f.rec.Pending())
	}
}

func TestExtensions_MismatchClearsReady(t *testing.T) {
	cases := map[string]struct {
		declared []migrate.Extension
		matched  sqltest.Response
		mismatch sqltest.Response
	}{
		"pending":         {vector, extCurrent, extMissing},
		"removed pending": {[]migrate.Extension{{Name: "vector", Removed: true}}, extState(nil, "0.8.0", false), extCurrent},
		"unavailable":     {vector, extCurrent, extUnavailable},
		"no update path":  {vector, extCurrent, extNoPath},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newExtensionService(t, c.declared, admin.Options{},
				slices.Concat(current(), []sqltest.Response{c.matched}, current(), []sqltest.Response{c.mismatch})...)
			ctx := context.Background()
			if st, err := f.service.Status(ctx); err != nil || !st.Ready || !f.service.Ready() {
				t.Fatalf("matching Status = %+v, %v; ready %v", st, err, f.service.Ready())
			}
			st, err := f.service.Status(ctx)
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if st.Ready || f.service.Ready() {
				t.Errorf("ready over a mismatched declaration: %+v", st)
			}
		})
	}
}

func TestExtensions_FaultIsAConflict(t *testing.T) {
	faults := map[string]struct {
		state    sqltest.Response
		sentinel error
	}{
		"unavailable":    {extUnavailable, migrate.ErrExtensionUnavailable},
		"no update path": {extNoPath, migrate.ErrNoUpdatePath},
	}
	conflict := func(t *testing.T, verb string, err, sentinel error) {
		t.Helper()
		if !errors.Is(err, admin.ErrConflict) || !errors.Is(err, sentinel) {
			t.Errorf("%s = %v, want ErrConflict and %v", verb, err, sentinel)
		}
	}
	for name, fault := range faults {
		t.Run(name+"/start", func(t *testing.T) {
			f := newExtensionService(t, vector, admin.Options{}, fault.state)
			conflict(t, "Start", f.service.Start(context.Background()), fault.sentinel)
			if f.service.Ready() || len(f.rec.SQL(sqltest.OpExec)) != 0 {
				t.Error("a faulted Start reported ready or wrote")
			}
		})
		t.Run(name+"/verify", func(t *testing.T) {
			f := newExtensionService(t, vector, admin.Options{}, extCurrent, exists(true), applied(), fault.state)
			ctx := context.Background()
			if err := f.service.Verify(ctx); err != nil || !f.service.Ready() {
				t.Fatalf("clean Verify = %v, ready %v", err, f.service.Ready())
			}
			conflict(t, "Verify", f.service.Verify(ctx), fault.sentinel)
			if f.service.Ready() {
				t.Error("ready after a faulted Verify")
			}
		})
		t.Run(name+"/up", func(t *testing.T) {
			f := newExtensionService(t, vector, admin.Options{}, slices.Concat(
				current(), []sqltest.Response{extCurrent},
				[]sqltest.Response{locked, created, applied(), fault.state, unlocked},
				current(), []sqltest.Response{fault.state},
			)...)
			ctx := context.Background()
			if st, err := f.service.Status(ctx); err != nil || !f.service.Ready() {
				t.Fatalf("clean Status = %+v, %v", st, err)
			}
			_, err := f.service.Up(ctx)
			conflict(t, "Up", err, fault.sentinel)
			if f.service.Ready() || f.rec.Pending() != 0 {
				t.Errorf("ready %v, pending responses %d", f.service.Ready(), f.rec.Pending())
			}
		})
	}
}

func TestExtensions_StartLogsPendingBeforeApplying(t *testing.T) {
	var buf bytes.Buffer
	f := newExtensionService(t, vector, admin.Options{Logger: slog.New(slog.NewTextHandler(&buf, nil))}, slices.Concat(
		[]sqltest.Response{extMissing},
		current(), []sqltest.Response{extMissing},
		[]sqltest.Response{locked, created, applied(), extMissing, {}, {}, unlocked},
		[]sqltest.Response{extCurrent}, current(),
	)...)
	if err := f.service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !f.service.Ready() || f.rec.Pending() != 0 {
		t.Errorf("ready = %v, pending responses = %d", f.service.Ready(), f.rec.Pending())
	}
	want := `msg="extension pending; applying" extension=vector installed="" default=0.8.0`
	if !strings.Contains(buf.String(), want) {
		t.Errorf("log lacks %s:\n%s", want, buf.String())
	}
	if execs := f.rec.SQL(sqltest.OpExec); !slices.Contains(execs, `CREATE EXTENSION IF NOT EXISTS "vector"`) {
		t.Errorf("execs = %q", execs)
	}
}
