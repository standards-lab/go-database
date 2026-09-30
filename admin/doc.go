// Package admin is the database admin service: schema state, verification,
// correction, seeding, named states, and diagnostics as operations over the
// sqlate library's functions, run once at startup and on demand from an
// administrative surface. The service owns the operations and their policy;
// the migration sets, the seeder with its named sets, the pattern catalog,
// and the statements registry are the consumer's, passed in at
// construction. A wiring defect panics, per the database package.
//
// # Migration sets
//
// One service administers every set its migrator runs, declared
// bottom-first: a set comes before the sets that build on it, as a
// library's shipped set comes beneath the consumer's own. A read or a
// whole-schema operation covers every set in declared order; a verb that
// targets one set takes its name. The migrator's ordering holds: a set is
// not reverted while a set above it has applied migrations, nor applied
// while a set below it has pending ones.
//
// # Startup
//
// The consumer declares the service at the stage its own stage table
// gives it, after the stage that starts the pool and before whatever needs
// the corrected schema:
//
//	lc.Add(lifecycle.Service{Name: "schema", Stage: stageSchema, Start: svc.Start, Check: svc})
//
// [Service.Start] applies pending migrations, verifies the seeder, and
// applies the configured seed set, idempotently at every start. A dirty
// set, or a history a set does not carry, fails startup, and a failed Start
// stops the process, so an operator repairs the schema through the verbs of
// another replica or starts a process against the corrected database.
// [Service.Ready] reports the schema alone, as the last operation that
// determined it found it; once Start has succeeded, a not-ready service
// re-verifies at most once per five seconds. A true Ready is not
// re-checked.
//
// # Operations
//
// Each verb triggers the library function of its name, and the mutating
// ones return the refreshed [Status]; [Service.Reset] is the one transition
// to a named state from any other. Reset, Down, and Force are destructive,
// and the administrative surface, application code, decides who may call
// them.
//
// # Errors
//
// A request outside a verb's domain is refused before any I/O:
// [ErrValidation], [ErrUnknownSet], [ErrUnknownState], or [ErrSeedDisabled].
// An operation the schema's state refuses is [ErrConflict], wrapping the
// migrate sentinel.
//
// # Dirty-set repair
//
// A non-transactional migration that fails partway leaves its set dirty,
// and every write refuses until [Service.Force] clears it. The operator
// fixes the failed migration's objects by hand, forces the set to the
// version that is applied, and runs [Service.Up].
package admin
