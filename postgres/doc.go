// Package postgres is the PostgreSQL provider: [New] constructs the pool
// over pgx's database/sql adapter from a finalized database.Config, without
// I/O. Port defaults to 5432, a host starting with "/" is a Unix-socket
// directory, and the password is set on the parsed config, never in the
// connection URL. An empty User falls back to the OS username. Every
// connection returns timestamptz values in time.UTC, whatever time.Local or
// the session's TimeZone is; timestamp values, which carry no zone, come
// back in time.UTC too. The provider supplies no dialect; the
// sqlate/postgres package does.
package postgres
