// Package postgres is the PostgreSQL provider: [New] constructs the pool over
// pgx's database/sql adapter from a finalized database.Config, without I/O.
// Port defaults to 5432, a host starting with "/" is a Unix-socket directory,
// and the password is set on the parsed config, never in the connection URL.
package postgres
