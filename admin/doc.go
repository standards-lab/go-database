// Package admin is the database admin service: schema state, verification,
// correction, seeding, named states, and diagnostics as operations over
// the sqlate library's functions, run once at startup and on demand from
// an administrative surface. The service owns the operations and their
// policy, such as which seed set applies at startup, and none of the
// content it administers: the migration sets, the seeder with its named
// sets, the pattern catalog, and the statements registry are the
// consumer's, passed in at construction.
//
// # Construction
//
// [New] takes four collaborators:
//
//   - the pool's lifecycle object
//   - the sqlate session over the same pool
//   - the migrator the consumer built over its migration sets
//   - the pattern catalog every statement compiles against
//
// [Options] carries the optional collaborators: a [Seeder], a [Registry], a
// logger, and the name of the seed set that applies at startup. A nil
// required collaborator, or a startup seed set without a seeder, is a
// wiring defect and panics, per the database package's wiring rule.
//
// # Migration sets
//
// The migrator runs one or more migration sets, declared bottom-first, each
// over its own history table: a library's shipped set beneath the
// consumer's own, for one. One service administers every set the migrator
// runs. A read or a whole-schema operation covers every set in declared
// order; a verb that targets one set takes its name, and an empty or
// undeclared name is [ErrUnknownSet], refused before any I/O. A migrator
// over one set is the same service with one entry in each report.
//
// # Startup
//
// [Service.Register] declares the service on a lifecycle coordinator at
// [Stage], after the pool at stage 0 and before the domains verify their
// statements at stage 2. [Service.Start] verifies every set's history; when
// migrations are pending, it logs them set by set, applies them under the
// migrator's lock, and verifies again. It then verifies the seeder's
// statements and applies the configured seed set when there is one:
// idempotent, so a deployment initializes its data at its first start and
// every later start leaves it as it is. A state the mechanism cannot
// correct — a dirty row, or a history a set does not carry — fails
// startup. An operator resolves it through the verbs. [Service.Ready]
// reports a clean, complete history in every set as of the last operation,
// so a readiness probe aggregating the service reflects it.
//
// # Operations
//
// Every operation is a trigger over a library function:
//
//   - [Service.Verify], [Service.Status], and [Service.Up] call the
//     migrator's verb of the same name over every set; [Status] carries
//     one [SetStatus] per set.
//   - [Service.Down], [Service.Steps], and [Service.Force] call the verb of
//     the same name on the set they name. The migrator's ordering holds: a
//     set is not reverted while a set above it has applied migrations, nor
//     applied while a set below it has pending ones.
//   - The mutating verbs return the refreshed [Status].
//   - [Service.States] lists the seeder's declared names.
//   - [Service.Seed] applies a named seed set, or the configured one, over
//     the schema as it stands.
//   - [Service.Reset] reverts every migration set, the last declared first,
//     dropping each history table; applies every set again; and seeds the
//     named state's set — the one transition that brings a database to a
//     named state from any other.
//   - [Service.Catalog] and [Service.Statements] read the pattern catalog
//     and the statements registry without I/O.
//   - [Service.Diagnose] pings the pool, reads the server's version through
//     the dialect's [Versioner] capability when it has one, and reports the
//     pool's counters.
//
// A seed operation without a seeder or a set is [ErrSeedDisabled]; an
// undeclared name is [ErrUnknownState]. Reset is destructive, in the class
// of Down and Force; the administrative surface decides who may call it,
// and asks for confirmation. The HTTP half, a route group over these
// methods, is application code.
//
// # Dirty-set repair
//
// A non-transactional migration that fails partway leaves its set's head
// dirty, and every run that writes refuses until it is cleared. Force is
// the repair: the operator fixes the failed migration's objects by hand,
// forces the named set to the version that is applied, then runs Up.
// Force touches no schema and checks no history first, since the set it
// repairs is the dirty one. After a revert refused partway through, Status
// is what tells the operator where each set stands.
package admin
