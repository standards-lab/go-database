// Package pgvector resolves the pgvector distance metric an application's
// configuration names into sqlate's metric table:
//
//   - [Metric], sqlate's pgvector metric, under this package's name
//   - [ParseMetric], which resolves a metric name to its [Metric]
//   - [ErrUnknownMetric], the error a name outside the table wraps
//
// The package imports only the standard library and sqlate's
// postgres/pgvector, and declares no type of its own. Importing this module
// is limited the way a provider's is: the composition root, the migrations,
// and the packages the application's design documentation declares.
//
// # Vectors
//
// Those packages use pgvector-go (github.com/pgvector/pgvector-go)
// directly; this package does not import it. Domain types hold []float32.
// pgvector.NewVector wraps one for a parameter, and a nullable column scans
// into sql.Null[pgvector.Vector], whose Slice returns the []float32:
//
//	import (
//		"database/sql"
//
//		"github.com/pgvector/pgvector-go"
//	)
//
//	_, err := db.ExecContext(ctx, "UPDATE doc SET embedding = $1 WHERE id = $2",
//		pgvector.NewVector(d.Embedding), d.ID)
//
//	var embedding sql.Null[pgvector.Vector]
//	if err := row.Scan(&d.ID, &embedding); err != nil {
//		return err
//	}
//	if embedding.Valid {
//		d.Embedding = embedding.V.Slice()
//	}
//
// # Metrics from configuration
//
// A metric named in configuration resolves with [ParseMetric] before any
// I/O; its OpClass builds the index and its Operator orders the query:
//
//	import vecmetric "github.com/standards-lab/go-database/postgres/pgvector"
//
//	metric, err := vecmetric.ParseMetric(cfg.Metric)
//	if err != nil {
//		return err
//	}
//	index := "CREATE INDEX doc_embedding ON doc USING hnsw (embedding " + metric.OpClass + ")"
//	nearest := "SELECT id FROM doc ORDER BY embedding " + metric.Operator + " $1 LIMIT 5"
package pgvector
