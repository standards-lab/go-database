// Package postgres is the PostgreSQL provider for the database package: it
// constructs the connection pool over pgx's database/sql adapter. The pgx
// dependency lives in this sub-module's go.mod, so it enters a consumer's
// graph only when this package is imported, once, at the composition root.
// The provider supplies no dialect: a composition root takes the dialect
// from the sqlate/postgres package and wraps the pool with sqlate.Wrap.
//
// # Construction
//
// [New] builds the pool from a finalized database.Config without I/O: it
// composes a postgres:// URL with net/url from the host, port (defaulting to
// 5432), name, user, and Options, parses it eagerly with pgx.ParseConfig, so
// a malformed config is a construction error rather than a first-query
// surprise, and then sets the password and connect timeout as fields on the
// parsed config. The password never enters the composed URL, so no character
// in it can break or leak through the connection string. An empty user falls
// back to pgx's default, the OS username. Options keys that name connection
// fields (host, port, user, password, dbname, database, connect_timeout) are
// rejected; values pgx cannot parse, such as an invalid sslmode, fail with
// the parse error.
//
// The result is a *database.DB: lifecycle wiring, readiness, and everything
// else the base package offers come with it. An unfinalized config is a
// wiring defect and panics; a rejected option is a content defect and
// returns an error, per the base package's wiring rule.
package postgres
