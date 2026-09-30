// Package database wraps a database/sql connection pool with lifecycle hooks,
// readiness, and the [Config] block that sizes it. Drivers live in provider
// sub-modules (postgres) that construct the pool and call [New]; sqlate owns
// the SQL layer above it, and the composition root wraps [DB.Conn] with
// sqlate.Wrap. A wiring defect, such as an unfinalized config, panics; a
// defect in configuration content is an error. A composition root declares
// the pool as one lifecycle service:
//
//	lc.Add(lifecycle.Service{Name: "database", Start: db.Start, Shutdown: db.Shutdown, Check: db})
package database
