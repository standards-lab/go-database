# go-database

The SQL infrastructure library of Go Elemental, the Standards Lab organization's Go
implementation of the Elemental Architecture: the database service in four layers, with the driver isolated in the `postgres`
provider sub-module.

The design and conventions of this repository are documented in the organization's
[documentation landing zone](https://github.com/standards-lab/docs); this context records only
working knowledge the landing zone and the code do not express. The repository page is
[go-database](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/index.md),
under the [Go Elemental](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/index.md)
standard, with the design detailed in
[Service tiers in SQL](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/tiers.md),
[The layer ontology](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/layers.md),
[The dialect interface](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/dialect.md),
and
[Providers](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/providers.md).

## Capability map

The built packages are authoritative through their code and `doc.go`; the landing zone documents
the design. Detail for what is unbuilt is added when it is about to be built.

- **database** — the service layer: the wrapper over the pool, the `Dialect` interface, the
  `Session` and `Tx` types with `ExecTx`, the configuration block, and the error types
  (connectivity, the constraint classes, version mismatch). Built. In v0.4 (planned) the
  package keeps `Config`, the pool, lifecycle, and readiness; `Session`, `Tx`, `Dialect`, and
  the error types move to `sqlate`, which maps every error inside the session.
- **ast** — the statement layer: standard SQL as values — expressions, predicates, tables,
  `Select`/`Compound` as the sealed `Query` expressions, `Insert`/`Update`/`Delete` as the
  write statements — rendered to `SQL` values through the dialect, with the two-direction
  capability pattern (`PagingRenderer` override, `ReturningRenderer` declared-native). Built;
  retires in v0.4 under the DSL strategy.
- **operation** — the contract layer: `Projection` with the `List` and `One` queries and the
  `Directives` request vocabulary; `Insertion` and the guarded commands on the
  optimistic-concurrency contract; the shared `Field` vocabulary and the `RecursivePath`
  computed-field pattern. Built; retires in v0.4, its directive vocabulary and typed errors
  salvaged into `query`.
- **exec** — the execution layer: `Scan`, the query runners over `Session`, the command
  runners over `*Tx` with guard-outcome mapping, every driver error through `MapError`. Built;
  retires in v0.4, its scan and guard-outcome logic salvaged into `query`.
- **seed** — reference-data loading: a runner over the consumer's seed file system, one
  transaction per step, the decode format selected by extension. Built; retires in v0.4 to a
  documented consumer pattern over the transaction runner.
- **postgres** — the PostgreSQL provider: the pool over pgx's `database/sql` adapter and the
  postgres dialect (`$N` placeholders, class-23 constraint classification, the `RETURNING`
  capability). Built. In v0.4 it keeps the driver, the DSN, and pool construction and
  supplies no dialect; the dialect is `sqlate/postgres`.
- **admin** — planned for v0.4: the database admin service over `sqlate`, generic over a
  migrator, a seeder, a catalog, and the pool. It verifies, migrates, and seeds at startup and
  on demand; every operation is a trigger over a library function.

The mechanism over authored SQL files (`query`, `migrate`, the scripted driver for unit tests,
and the lint) is `sqlate`, a standalone library below this one, settled by the
`v1.data.sql.prototype` experiment and released at v0.1.0 (`github.com/standards-lab/sqlate`).
`design/layers.md` describes v0.3.0 as built and is superseded; `concepts/sql-architecture.md`
is the plan the experiment started from, with dated notes where the outcome differed. The
`v1.data.sql.integration.database` task rewrites both.
