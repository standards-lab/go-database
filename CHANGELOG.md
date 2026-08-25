# Changelog

All notable changes to `github.com/standards-lab/go-database` are documented here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the module adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). This changelog covers the base module
only; the `postgres` sub-module keeps its own.

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
