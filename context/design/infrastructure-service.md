# The infrastructure service

Settled by the `v1.data.sql.integration.database` session (2026-09-04), which built v0.4.0
over `sqlate` v0.1.0. The built packages are authoritative for everything they express; this
note holds the boundaries the code cannot state on its own: what this module owns against
`sqlate` below it and the application above it, and the composition a root writes across the
three. The landing zone's go-database pages restate it once the docs pass rewrites them.

## What each party owns

- **go-database** owns the pool and its administration. The `database` package constructs
  nothing itself: a provider sub-module builds the `*sql.DB` from the configuration block, and
  the package wraps it with lifecycle and readiness. The `admin` package administers the schema
  over that pool and owns only operations and policy, never content.
- **sqlate** owns everything from the `.sql` file to the scanned row: the session over the pool,
  the dialect and its error classification, transactions, the pattern catalog, compiled
  statements, and the migrator. go-database imports its base module for the `admin` package and
  never its engine sub-module; the composition root imports `sqlate/postgres`.
- **The application** owns the content the admin service administers (the migration set, the
  seeder, the statements registry) and the HTTP half of the admin surface (the route group, the
  handler, the mount). The template scaffolds that half; a management listener is the
  `v1.data.sql.integration.listener` task.

The dialect capabilities follow the same line. The `admin` package declares the `Versioner`
interface it needs and asserts it off the session's dialect, the way `migrate` asserts its
`Catalog`; `sqlate/postgres` implements it (`ServerVersion`, postgres/v0.1.1). The engine's
native text lives with the engine, and the consumer of a capability declares its interface.

## The composition a root writes

```go
db, err := postgres.New(cfg.Database)            // the pool, no I/O
lc.Add(lifecycle.Service{Name: "database", Stage: 0, Start: db.Start, Shutdown: db.Shutdown, Check: db})

sdb := sqlate.Wrap(db.Conn(), pgdialect.Dialect{})       // the session and the dialect
catalog := query.MustCatalog(query.Patterns(), data.Patterns())
m, err := migrate.New(sdb, data.Migrations(), migrate.Options{Logger: logger})

svc := admin.New(db, sdb, m, catalog, admin.Options{Seed: cfg.Admin.SeedEnabled(), Seeder: seeder, Registry: registry, Logger: logger})
svc.Register(lc)                                  // stage 1: verify, apply, verify, seed
```

The lifecycle stages are the ordering rule: the pool at stage 0, the schema at stage 1, and the
domains at stage 2, where each verifies its own statements against the migrated schema.
Readiness composes the same way: the pool reports live connectivity, and the admin service
reports a clean, complete history as of its last operation.

## The wiring rule

A defect the composition root wires panics; a defect in configuration content returns an error.
`database.New` panics on an unfinalized config or a nil pool; `admin.New` panics on a nil
collaborator or the seed switch without a seeder; `postgres.New` panics on an unfinalized config
and returns an error for a reserved connection option. The rule is stated in the `database`
package comment and applied in all three packages.

## Assumptions

- Assumes the `admin` package stays free of `net/http`: an administrative surface other than the
  route group (a CLI mode, a signal) would call the same methods.
- Assumes one migration set per admin service. A service administering several schemas would
  register several services under distinct lifecycle names.
