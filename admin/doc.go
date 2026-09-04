// Package admin is the database admin service: schema state, verification,
// correction, seeding, and diagnostics as operations over the sqlate
// library's functions, run once at startup and on demand from an
// administrative surface. The service owns the operations and their
// policy, such as when the seed runs, and none of the content it
// administers: the migration set, the seeder, the pattern catalog, and the
// statements registry are the consumer's, passed in at construction.
//
// # Construction
//
// [New] takes the pool's lifecycle object, the sqlate session over the same
// pool, the migrator the consumer built over its migration set, and the
// pattern catalog every statement compiles against. [Options] carries the
// optional collaborators: a [Seeder], a [Registry], a logger, and the seed
// switch. A nil required collaborator, or the seed switch without a seeder,
// is a wiring defect and panics, per the database package's wiring rule.
//
// # Startup
//
// [Service.Register] declares the service on a lifecycle coordinator at
// [Stage], after the pool at stage 0 and before the domains verify their
// statements at stage 2. [Service.Start] verifies the migration history,
// applies a pending set under the migrator's lock and verifies again,
// verifies the seeder's statements, and seeds when the switch is on. A
// state the mechanism cannot correct, a dirty row or a history the set does
// not carry, fails startup; an operator resolves it through the verbs.
// [Service.Ready] reports a clean, complete schema as of the last
// operation, so a readiness probe aggregating the service reflects it.
//
// # Operations
//
// Every operation is a trigger over a library function. [Service.Verify],
// [Service.Status], [Service.Up], [Service.Down], [Service.Steps], and
// [Service.Force] call the migrator's verb of the same name; the mutating
// verbs return the refreshed [Status]. [Service.Seed] runs the seeder when
// the switch is on and returns [ErrSeedDisabled] otherwise. [Service.Catalog]
// and [Service.Statements] read the pattern catalog and the statements
// registry without I/O. [Service.Diagnose] pings the pool, reads the server's
// version through the dialect's [Versioner] capability when it has one, and
// reports the pool's counters. The HTTP half, a route group over these
// methods, is application code.
package admin
