// Package database wraps a database/sql connection pool with lifecycle
// hooks, readiness, and the [Config] block that sizes it. Drivers live in
// provider sub-modules (postgres) that construct the pool and call [New];
// sqlate owns the SQL layer above the pool, and the composition root wraps
// [DB.Conn] with sqlate.Wrap. A wiring defect, such as an unfinalized
// config, panics; a defect in configuration content is an error.
//
// # Configuration
//
// [Config] holds the connection identity, pool sizing, and timeouts.
// [Config.Merge] overlays another block's set fields, [Config.Finalize]
// applies defaults and environment overrides and validates, and
// [Config.Finalized] reports whether [New] can use the result. [NewEnv]
// composes the override names from a prefix into an [Env], which Finalize
// records on the config.
//
// # Pool
//
// [New] wraps a provider-constructed pool in a [DB]. [DB.Start] and
// [DB.Shutdown] are its lifecycle hooks, [DB.Ready] and [DB.Ping] check
// connectivity, [DB.Conn] returns the pool, and [DB.ConnTimeout] returns
// the bound Start and Ready apply to a ping.
//
// A composition root defines the DB as a node of a go-core graph and hands
// the built System to a lifecycle Coordinator. The DB is a lifecycle
// Starter, Stopper, and ReadinessChecker, so the Coordinator starts it in
// its layer, shuts it down, and lists its readiness among its Checks under
// the node's name. The DB needs no adapter:
//
//	g := graph.New()
//	db := g.Define("database", func(*graph.Scope) (*database.DB, error) {
//		return postgres.New(cfg)
//	})
//	// The nodes that query the database call s.Use(db).
//	sys, err := g.Build(roots...)
//	if err != nil {
//		return err
//	}
//	return lifecycle.New(sys, lifecycleCfg).Run(ctx)
//
// The Coordinator also calls Shutdown after a failed Start, which
// [DB.Shutdown] allows.
//
// # Errors
//
// [ErrNotReady] reports a call before Start or after Shutdown.
// [ErrConnectionFailed], which is sqlate's sentinel, reports a failure to
// reach the database.
package database
