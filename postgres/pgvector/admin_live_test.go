//go:build integration

package pgvector_test

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/standards-lab/go-database"
	"github.com/standards-lab/go-database/admin"
	"github.com/standards-lab/sqlate/migrate"
	"github.com/standards-lab/sqlate/postgres/pgtest"
	sqlvector "github.com/standards-lab/sqlate/postgres/pgvector"
	"github.com/standards-lab/sqlate/query"
)

const dsnEnv = "GO_DATABASE_DSN"

var declared = migrate.Options{Extensions: []migrate.Extension{{Name: sqlvector.Extension}}}

func vectorSet(up, down string) []migrate.Set {
	return []migrate.Set{{Name: "vec", Table: "vec_history", Migrations: []migrate.Migration{
		{Version: 1, Name: "table", Up: up, Down: down, Transactional: true},
	}}}
}

func text(t testing.TB, pool *sql.DB, q string, args ...any) string {
	t.Helper()
	var s string
	if err := pool.QueryRowContext(context.Background(), q, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return s
}

func installedVersion(t testing.TB, pool *sql.DB) string {
	t.Helper()
	return text(t, pool, "SELECT COALESCE((SELECT extversion FROM pg_extension WHERE extname = $1), '')", sqlvector.Extension)
}

func imageDefaultVersion(t testing.TB, pool *sql.DB) string {
	t.Helper()
	v := text(t, pool, "SELECT COALESCE((SELECT default_version FROM pg_available_extensions WHERE name = $1), '')", sqlvector.Extension)
	if v == "" {
		t.Fatalf("the image does not offer %s", sqlvector.Extension)
	}
	return v
}

func TestLive_AdminStartActivatesVectorAtTheImageDefault(t *testing.T) {
	ctx := context.Background()
	d := pgtest.Create(t, dsnEnv, "vec")
	if got := installedVersion(t, d.Pool); got != "" {
		t.Fatalf("vector in the scratch database is at %q before Start, want it absent", got)
	}
	def := imageDefaultVersion(t, d.Pool)

	db := d.Session()
	m, err := migrate.New(db, vectorSet("CREATE TABLE doc (id integer PRIMARY KEY, embedding vector(3))", "DROP TABLE doc"), declared)
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	cfg := database.Config{Name: "test"}
	if err := cfg.Finalize(""); err != nil {
		t.Fatal(err)
	}
	s := admin.New(database.New(d.Pool, cfg), db, m, query.MustCatalog(query.Patterns()), admin.Options{})
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := installedVersion(t, d.Pool); got != def {
		t.Errorf("vector after Start is at %q, want the image default %q", got, def)
	}
	st, err := s.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	want := []admin.ExtensionStatus{{Name: sqlvector.Extension, Installed: def, Default: def}}
	if !slices.Equal(st.Extensions, want) {
		t.Errorf("Status.Extensions = %+v, want %+v", st.Extensions, want)
	}
	if !st.Ready || !s.Ready() {
		t.Errorf("Status.Ready = %v, Ready() = %v, want both true", st.Ready, s.Ready())
	}
}
