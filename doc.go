// Package database is the SQL infrastructure service: a lifecycle-integrated
// wrapper over a database/sql connection pool and the configuration block
// that sizes it. The package depends on the standard library and go-core's
// config package alone; every driver lives in a provider sub-module
// (postgres) that constructs the pool, so a consumer imports its provider
// once, at the composition root. Statements, sessions, transactions, and
// the dialect are the sqlate library's: the composition root wraps
// [DB.Conn] with sqlate.Wrap and the engine's dialect, and the admin package
// administers the schema over both.
//
// # Wrapper
//
// [New] wraps the provider-constructed *sql.DB with a finalized [Config],
// applying the config's pool settings, the half of construction every
// provider shares. It performs no I/O: an unfinalized Config or a nil conn
// panics with the fix named, and everything else waits for Start.
// [DB.Conn] exposes the pool.
//
// The panic follows the package's wiring rule. A defect the composition
// root wires, such as an unfinalized config or a missing pool, panics,
// because no runtime condition produces it and no caller can recover from
// it sensibly. A defect in configuration content, such as a reserved
// connection option, returns an error from the provider's constructor,
// because configuration is input.
//
// # Lifecycle wiring
//
// The package registers no lifecycle hooks of its own. [DB.Start] and
// [DB.Shutdown] carry the lifecycle package's hook signature, so a
// composition root wires the database as bare method values:
//
//	lc.OnStartup(db.Start)
//	lc.OnShutdown(db.Shutdown)
//
// Start establishes connectivity with a ping bounded by the configured
// conn_timeout, so a dead database fails the coordinator's startup rather
// than serving traffic unready. Shutdown closes the pool and returns the
// close error; after a failed startup the drain passes through it cleanly.
//
// # Readiness
//
// [DB.Ready] satisfies lifecycle.ReadinessChecker structurally and reports
// live connectivity: false before Start or after Shutdown, and otherwise a
// ping bounded by conn_timeout. A readiness probe aggregating the database
// therefore reflects it now, 503 during an outage and healed when the
// database returns, at the cost of one bounded round trip per probe.
// [DB.Ping] is the same verification under the caller's context and bound.
//
// # Configuration
//
// [Config] holds the connection identity, pool sizing, and timeouts, and
// implements the config package's Merge and Finalize contract, so it loads as
// part of an application's configuration. The numeric and duration fields are
// pointers: nil is unset and takes the default, while an explicit zero
// survives the load and means what it says. Port defaults in the provider,
// User and Password stay optional (requiredness varies by provider and auth
// mode), and Options passes engine-specific connection keys through to the
// provider. Finalize composes the standard override names from the prefix it
// receives (via [NewEnv], recorded on [Env] for introspection); an empty
// prefix disables the overrides.
//
// # Errors
//
// [ErrNotReady] and [ErrConnectionFailed] classify the service conditions,
// wrapped in the dual form fmt.Errorf("%w: %w", sentinel, err) so errors.Is
// classifies while the driver's error stays recoverable. Errors a statement
// raises, the constraint classes among them, are classified by the sqlate
// dialect inside the session and are sqlate's sentinels.
package database
