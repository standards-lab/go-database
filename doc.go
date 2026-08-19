// Package database provides the SQL data layer's dialect-neutral core: a
// lifecycle-integrated wrapper over a database/sql connection pool and the
// dialect seam its providers implement. The package is stdlib-plus-config
// only; every driver lives in a provider sub-module (database/postgres) that
// constructs the pool and supplies the dialect, so a consumer imports its
// provider once, at the composition root.
//
// # Wrapper
//
// [New] wraps the provider-constructed *sql.DB with the provider's [Dialect]
// and a finalized [Config], applying the config's pool settings — the
// dialect-independent half of construction every provider shares. It performs
// no I/O: an unfinalized Config, nil conn, or nil dialect panics with the fix
// named, and everything else waits for Start. [DB.Conn] exposes the pool;
// [DB.Dialect] the seam.
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
// therefore reflects it now — 503 during an outage, healed when the database
// returns — at the cost of one bounded round trip per probe. [DB.Ping] is the
// same verification under the caller's context and bound.
//
// # Dialect
//
// [Dialect] is the whole provider surface the base needs: a name, the bind
// placeholder renderer, and the driver error mapper. At the connectivity
// slice MapError is the identity; query surfaces in later slices route SQL
// rendering and error classification through the seam, which is what keeps
// them dialect-neutral. Providers are selected by typed construction — a
// [Provider] constant and a constructor per sub-module, no registry.
//
// # Configuration
//
// [Config] holds the connection identity, pool sizing, and timeouts, and
// implements the config package's Merge and Finalize contract, so it loads as
// part of an application's configuration. The numeric and duration fields are
// pointers: nil is unset and takes the default, while an explicit zero
// survives the load and means what it says. Port defaults in the provider,
// User and Password stay optional (requiredness varies by provider and auth
// mode), and Options passes dialect-specific connection keys through to the
// provider. Finalize composes the standard override names from the prefix it
// receives (via [NewEnv], recorded on [Env] for introspection); an empty
// prefix disables the overrides.
//
// # Errors
//
// [ErrNotReady] and [ErrConnectionFailed] are the package's two sentinels,
// wrapped in the dual form fmt.Errorf("%w: %w", sentinel, err) so errors.Is
// classifies while the driver's error stays recoverable. sql.ErrNoRows is
// never mapped; it flows to the boundary unchanged.
package database
