# Changelog

All notable changes to `github.com/standards-lab/go-database` are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the module adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). This changelog covers the base module
only; the `postgres` sub-module keeps its own.

## [Unreleased]

## [v0.4.0] - 2026-09-04

The module is reduced to the SQL infrastructure service over the `sqlate` library
(`github.com/standards-lab/sqlate`), which owns everything from the `.sql` file to the scanned
row. A composition root wraps the pool this module constructs with `sqlate.Wrap` and the engine's
dialect from `sqlate/postgres`, compiles its statements through `sqlate/query`, and migrates
through `sqlate/migrate`.

### Changed

- **Breaking:** the `ast`, `operation`, `exec`, and `seed` packages are removed. Authored SQL
  files over `sqlate/query` replace the statement vocabulary and the runners; `sqlate/migrate`
  and a consumer's own seed statements replace `seed`.
- **Breaking:** the `Session`, `Tx`, and `Dialect` types, `ExecTx`, and the `Provider` constant
  are removed, with the `DB.Dialect`, `DB.Begin`, `DB.QueryContext`, `DB.QueryRowContext`, and
  `DB.ExecContext` methods. The `sqlate` package owns the session and the dialect.
- **Breaking:** the four constraint-class sentinels, `ConstraintError`, and `ErrVersionMismatch`
  are removed. The `sqlate` package owns the constraint classes and `sqlate/query` the version
  mismatch; the `sqlate/postgres` dialect classifies driver errors inside the session.
- **Breaking:** `New` is `New(conn *sql.DB, cfg Config) *DB`. It panics on an unfinalized
  config or a nil pool, the wiring rule the package comment now states.
- The `build` task and the CI build step run each module with `GOWORK=off`, so a pin that no
  longer builds fails rather than being masked by the committed `go.work`.

### Added

- `admin`: the database admin service. `New` takes the pool's lifecycle object, the `sqlate`
  session, a prebuilt `migrate.Migrator`, and the pattern catalog; `Options` carries a
  `Seeder`, a `Registry` of compiled statements, a logger, and the seed switch. `Register`
  declares the service at lifecycle `Stage` 1; `Start` verifies the history, applies a pending
  set, verifies the seeder, and seeds when enabled. `Verify`, `Status`, `Up`, `Down`, `Steps`,
  `Force`, and `Seed` are triggers over the library's functions; `Catalog` and `Statements`
  read build-time state; `Diagnose` pings, reads the server's version through the dialect's
  optional `Versioner` capability, and reports the pool's counters.
- The base module requires `github.com/standards-lab/sqlate` v0.1.0, a standard-library-only
  module.

## [v0.3.0] - 2026-08-28

### Changed

- **Breaking:** the `query` package is reorganized into the library's layer ontology. The
  statement layer is now `ast` — the standard-SQL AST and its renderer — and the contract
  layer moves to the new `operation` package: `Projection` (with `Field` and the builders),
  `Directives`, and the typed field-contract errors. Statements render with
  `Render(d) (ast.SQL, error)` in place of `SQL(d) (string, []any, error)`; `ast.SQL` is the
  rendered unit (`Text`, `Args`). The list operation is `Projection.List`, returning the
  `List` envelope (`Count`, `Page`) in place of `Statements`.
- **Breaking:** the `Query` interface narrows to the query expressions — `Select` and
  `Compound`, the statements valid inside a CTE, a derived table, a subquery, and
  `INSERT ... SELECT` — enforced at compile time; the write statements never implement it.
- Rendering validates before it emits, at every scope: each statement, clause, and loop
  iteration checks the invariants decidable at its entry before its first write, so the
  outermost defect wins under the first-failure rule.

### Added

- `ast` — the write statements `Insert`, `Update` (with `Assignment`), and `Delete`: plain
  table names, operand-lifting values, the same predicate trees for WHERE, rendering only at
  the outermost position. The `Returning` clause is the first declared-native feature:
  rendered only through the `ReturningRenderer` capability, failing with the typed
  `UnsupportedFeatureError` on a dialect without it — the capability pattern's second
  direction beside `PagingRenderer`'s standard-emission override. `Writer` gains `Column`;
  `Expression` gains `Empty`.
- `operation` — the query side's single-row read `Projection.One`, resolving the field and
  its typed unknown-field error at the library; the promoted structural builders `Columns`
  and `Fields`; the promoted `RecursivePath` computed-field pattern; and the command side:
  `Insertion` (the identity-returning insert), `GuardedUpdate` and `GuardedDelete` on the
  optimistic-concurrency contract, with `Guard` naming the consumer's version column and
  expected version, and the `Guarded` command-and-check envelope in pure standard SQL.
- `exec` — the execution layer, the only layer touching database/sql at runtime: `Scan`, the
  query runners `List`, `One`, and `Query` over the new `Session` seam, and the command
  runners `Insert` (returning `Identity`), `Update` (returning the deterministic new
  version), and `Delete`, mapping guard outcomes to `sql.ErrNoRows` and
  `ErrVersionMismatch` and routing every driver error through the dialect's `MapError`.
- `database` — the `Session` interface implemented by `DB` and the new `Tx`; `Begin`,
  `ExecTx` as the unit-of-work helper, and `Tx.Commit` mapping commit errors through the
  dialect, so violations deferred to COMMIT are classified. The error taxonomy grows the
  four constraint-class sentinels (`ErrUniqueViolation`, `ErrForeignKeyViolation`,
  `ErrCheckViolation`, `ErrNotNullViolation`), `ErrVersionMismatch`, and `ConstraintError`
  carrying the constraint name while unwrapping to both the class and the driver error.

## [v0.2.0] - 2026-08-25

### Added

- `query` — the persistence query vocabulary: standard SQL composed as Go values, rendered
  through the provider's `Dialect`, in two levels. The composition core is a sealed
  standard-SQL AST — expressions (`Col` preserving identifier parts, `Raw` as the deliberate
  escape hatch, `Val`, `Fn`), predicates with the full comparison suite (`Eq` through `Le`,
  `Like`, `In` with values or a subquery, null tests, `And`/`Or`/`Not`), joins, recursive
  CTEs, and set operations — with `Select` and `Compound` as struct values rendered by one
  walker: placeholders through `Dialect.Placeholder`, bound arguments in encounter order, and
  a structural defect wrapping the `ErrInvalidStatement` sentinel. Portability is structural:
  table aliases render without `AS`, compound branches without parentheses, `WITH` belongs to
  the outermost statement, and paging is the bound SQL:2008 `OFFSET/FETCH` form under a
  required ORDER BY. Where an engine lacks a standard form, an optional dialect extension
  renders it — `PagingRenderer`, with `Writer` as its surface, is the first — and the
  `Dialect` interface itself is unchanged. The read vocabulary sits on top: `Projection`
  (name-to-expression fields, one key field, hoisted CTEs) and `Directives` (1-based page,
  sorts, operator-keyed filters, plus a composed `Predicate` door) lower onto the core through
  `Projection.Statements`, which returns the count and page statements over one WHERE clause.
  An unknown field or operator is a typed error the consumer maps — `UnknownFieldError`,
  `UnknownOperatorError` — and the key field joins every ORDER BY as the tie-breaker, so
  offset paging is stable.

## [v0.1.1] - 2026-08-24

### Changed

- The go-core pin moves to v0.3.0, which adds the `process` package and builds on Go 1.27.
  Nothing in the module uses the new package; the pin is the committed steady state for
  consumers building on this release.
- The module builds on Go 1.27 (from 1.26), aligning it with the rest of the standard's
  modules.

## [v0.1.0] - 2026-08-19

The first release of the database capability: the `database` and `seed` packages. The module
depends on the standard library and `github.com/standards-lab/go-core v0.1.0`.

### Added

- `database` — the SQL data layer's dialect-neutral core. `database.New` wraps a
  provider-constructed `*sql.DB` with the provider's `Dialect` and a finalized `Config`, applying
  the pool settings; `Start` and `Shutdown` carry the `go-core` lifecycle hook signature and
  register as bare method values; `Ready` satisfies `lifecycle.ReadinessChecker` structurally
  with a live ping bounded by `conn_timeout`, so a readiness probe reflects the database now
  rather than its state at boot. Two sentinels, `ErrNotReady` and `ErrConnectionFailed`, wrap in
  the dual form `fmt.Errorf("%w: %w", sentinel, err)`; `sql.ErrNoRows` is never mapped. The
  `Dialect` interface (name, placeholder renderer, error mapper) is everything a provider
  implements for the base; `MapError` returns the error unchanged. `Config` keeps discrete
  fields so the password alone rides the secrets layer; `Port` has no base default, because the
  default port is a provider fact.
- `seed` — a seed runner over what only the consumer knows: the seed file system, a typed load
  function per table, and the step order. `Table` binds a seed file to a load function as an
  opaque `Step`; `Run` executes the steps in order, each in its own transaction, selecting the
  decode `Format` by file extension and logging each step applied. Formats register at
  construction, with no registry and no `init`; the strict `JSON` format ships with the package,
  so an unknown field or trailing content in a curated seed file fails the decode. Idempotency
  stays in the load function's SQL, where the conflict target is known.

[Unreleased]: https://github.com/standards-lab/go-database/compare/v0.4.0...HEAD
[v0.4.0]: https://github.com/standards-lab/go-database/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/standards-lab/go-database/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/standards-lab/go-database/compare/v0.1.1...v0.2.0
[v0.1.1]: https://github.com/standards-lab/go-database/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/standards-lab/go-database/releases/tag/v0.1.0
