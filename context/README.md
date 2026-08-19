# go-database

The database capability of `go-minimal`, the Standards Lab organization's minimal-dependency Go
standard. This is one capability repository of the standard: a reader who opens it sees the
organization's SQL data-access conventions and nothing else, and a consumer who takes it pulls one
capability. Built with the marathon workflow, so the workflow itself is exercised in the process.

## What we're building toward

- The lowest practical level of abstraction, no frameworks: plain SQL over `database/sql`, with a
  raw driver in the provider. Dependencies flow downward only; this repository sits on `go-core`
  and is unaware of what runs above it.
- Two tiers. The standard tier is exactly the technology's common standard, ISO/IEC 9075 SQL, as
  the `database` package in the base module; the native tier is the provider's own API, reached
  through the handle the capability exposes. A consumer interfaces at the resolution its purpose
  requires and wraps native use beneath the standard tier. See `design/tiers.md`.
- One base module, versioned and released as one artifact, with each provider a module of its own
  that pins its driver.

## Repository topology

The repository is one base module rooted at `github.com/standards-lab/go-database`, holding the
`database` package at its root and `seed` beside it, plus the `postgres` provider as a nested
sub-module. The base module depends on the standard library and `go-core`; a driver enters only
through a provider. See `design/topology-and-naming.md`.

## Capability map

The built packages are authoritative through their code and `doc.go`; detail for what is unbuilt
is added when it is about to be built.

- **database** — the dialect-neutral core: the `database.DB` wrapper over a provider-constructed
  pool, with `Start`, `Shutdown`, and `Ready` meeting the `go-core` lifecycle contracts; the
  `Dialect` interface providers implement; the configuration block that takes part in the layered
  load; and the error sentinels in the dual-wrapped form. Built.
- **seed** — reference-data loading: a runner over the consumer's seed file system, a typed load
  function per table, one transaction per step, the decode format selected by extension. Built.
- **postgres** — the PostgreSQL provider: constructs the pool over pgx's `database/sql` adapter and
  supplies the postgres dialect. Built.
- **Candidate direction** — the query vocabulary (`concepts/query.md`) and the write path
  (`concepts/writes.md`). Each begins with an evaluation of idiomatic patterns across the common
  engines, settling the standard and native sets and what `Dialect` grows to hold, and each is
  built in its own session.

## How this repository works

- **Tiers** — the standard and native tiers, the native tier at configuration and error level, the
  import boundary and the composition root. See `design/tiers.md`.
- **Dependencies** — what the base module may depend on, where a driver lives, what the base takes
  from `go-core`. See `design/dependencies.md`.
- **The dialect interface** — what it holds and how it grows. See `design/dialect.md`.
- **Providers** — typed construction, one provider per engine, what changing a provider costs,
  where a provider for another engine is defined. See `design/providers.md`.
- **Topology and naming** — the base module, the provider sub-modules, the module-path and tag
  conventions. See `design/topology-and-naming.md`.
- **Releases and CI** — root tags for the base and prefix tags for each provider, the per-module
  CI matrix, the `go.work` and `mise` workflow. See `design/release-and-ci.md`.
- **Tests and documentation** — co-located, black-box, hermetic tests; `doc.go` ownership. See
  `design/tests-and-docs.md`.
