// Package seed loads reference data into a database from seed files. The
// consumer supplies the parts only it knows — the seed file system, a typed
// load function per table, and the step order — and the runner owns the
// mechanics: format selection by file extension, decoding, one transaction
// per step, and the outcome log. The package is stdlib-only, so it lives in
// the base library beside database.
//
// # Steps
//
// [Table] binds one seed file to a typed load function, producing an opaque
// [Step]; [Runner.Run] executes the steps in the order given, each in its
// own transaction, stopping at the first failure with the step's path in the
// error. Idempotency belongs to the load function's SQL — INSERT … ON
// CONFLICT DO NOTHING against the table's natural key — because the conflict
// target is knowledge the consumer holds, not the runner.
//
// # Formats
//
// [Format] decodes one seed file and owns the file extensions it claims.
// Formats are passed to [New] at construction — typed registration, no
// registry, no init side effects — and no two may claim the same extension.
// [JSON] ships with the package and decodes strictly: an unknown field or
// trailing content is a defect in a curated seed file, not data to ignore.
// Another format is one Format implementation, supplied by a sub-module or
// by the consumer.
package seed
