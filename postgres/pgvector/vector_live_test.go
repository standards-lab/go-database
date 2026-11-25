//go:build integration

package pgvector_test

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/pgvector/pgvector-go"
	vecmetric "github.com/standards-lab/go-database/postgres/pgvector"
	"github.com/standards-lab/sqlate/postgres/pgtest"
	sqlvector "github.com/standards-lab/sqlate/postgres/pgvector"
)

func TestLive_VectorColumnRoundTripsValuesAndNull(t *testing.T) {
	ctx := context.Background()
	d := pgtest.MigratedWith(t, dsnEnv, "vec", declared,
		vectorSet("CREATE TABLE doc (id integer PRIMARY KEY, embedding vector(3))", "DROP TABLE doc")...)
	stored := map[int][]float32{1: {1, 2, 3}, 2: {-0.5, 0.25, 1e-3}, 3: nil}
	for id, v := range stored {
		var arg any
		if v != nil {
			arg = pgvector.NewVector(v)
		}
		if _, err := d.Pool.ExecContext(ctx, "INSERT INTO doc (id, embedding) VALUES ($1, $2)", id, arg); err != nil {
			t.Fatalf("insert %d: %v", id, err)
		}
	}
	for id, want := range stored {
		var got sql.Null[pgvector.Vector]
		if err := d.Pool.QueryRowContext(ctx, "SELECT embedding FROM doc WHERE id = $1", id).Scan(&got); err != nil {
			t.Fatalf("select %d: %v", id, err)
		}
		if got.Valid != (want != nil) {
			t.Errorf("row %d: Valid = %v, want %v", id, got.Valid, want != nil)
			continue
		}
		if got.Valid && !slices.Equal(got.V.Slice(), want) {
			t.Errorf("row %d: Slice() = %v, want %v", id, got.V.Slice(), want)
		}
	}
}

func TestLive_EachMetricOrdersItsOwnNearestRow(t *testing.T) {
	ctx := context.Background()
	d := pgtest.MigratedWith(t, dsnEnv, "vec", declared,
		vectorSet("CREATE TABLE item (name text PRIMARY KEY, embedding vector(2) NOT NULL)", "DROP TABLE item")...)
	rows := map[string][]float32{
		"aligned": {10, 0},
		"long":    {20, 15},
		"close":   {1, 0.5},
	}
	for name, v := range rows {
		if _, err := d.Pool.ExecContext(ctx, "INSERT INTO item (name, embedding) VALUES ($1, $2)", name, pgvector.NewVector(v)); err != nil {
			t.Fatalf("insert %s: %v", name, err)
		}
	}
	probe := pgvector.NewVector([]float32{1, 0})
	nearest := map[string]string{"l2": "close", "cosine": "aligned", "inner product": "long"}

	if got := len(sqlvector.Metrics()); got != len(nearest) {
		t.Fatalf("sqlate has %d metrics, the test expects a nearest row for %d", got, len(nearest))
	}
	for name, want := range nearest {
		t.Run(name, func(t *testing.T) {
			metric, err := vecmetric.ParseMetric(name)
			if err != nil {
				t.Fatalf("ParseMetric(%q): %v", name, err)
			}
			q := "SELECT name FROM item ORDER BY embedding " + metric.Operator + " $1 LIMIT 1"
			if got := text(t, d.Pool, q, probe); got != want {
				t.Errorf("%s nearest to %v = %q, want %q", q, probe.Slice(), got, want)
			}
		})
	}
}
