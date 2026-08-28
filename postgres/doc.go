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
// The result is a *database.DB: lifecycle wiring, readiness, and everything
// else the base package offers come with it, and the [Provider] constant
// supports typed selection in the composition root.
//
// # Dialect
//
// The dialect names itself "postgres" and renders bind placeholders as $1,
// $2, …. MapError classifies SQLSTATE class-23 constraint violations into
// the base package's sentinels — unique, foreign-key, check, not-null —
// through a ConstraintError carrying the violated constraint's name; every
// other error, sql.ErrNoRows included, passes through unchanged. The
// dialect's declared native reach beyond the standard core is the returning
// clause: it implements ast.ReturningRenderer, emitting the postgres
// RETURNING form for the write statements.
package postgres
