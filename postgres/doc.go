// Package postgres is the PostgreSQL provider for the database package: it
// constructs the connection pool over pgx's database/sql adapter and supplies
// the postgres dialect. The pgx dependency lives in this sub-module's go.mod,
// so it enters a consumer's graph only when this package is imported — once,
// at the composition root.
//
// # Construction
//
// [New] builds the pool from a finalized database.Config without I/O: it
// composes a postgres:// URL with net/url from the host, port (defaulting to
// 5432), name, user, and Options, parses it eagerly with pgx.ParseConfig — a
// malformed config is a construction error, not a first-query surprise — and
// then sets the password and connect timeout as fields on the parsed config.
// The password never enters the composed URL, so no character in it can break
// or leak through the connection string. An empty user falls back to pgx's
// default, the OS username. Options keys that name connection fields (host,
// port, user, password, dbname, database, connect_timeout) are rejected;
// values pgx cannot parse, such as an invalid sslmode, fail with the parse
// error.
//
// The result is a *database.DB: lifecycle wiring, readiness, and the query
// surface come from the base package, and the [Provider] constant supports
// typed selection at the composition root.
//
// # Dialect
//
// The dialect names itself "postgres", renders bind placeholders as $1, $2,
// …, and maps errors as the identity at the connectivity slice —
// classification of constraint violations arrives with the write surface.
package postgres
