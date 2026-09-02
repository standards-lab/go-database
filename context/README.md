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

- **database** — the service layer: the wrapper, the `Dialect` interface, the `Session`/`Tx`
  seam with `ExecTx`, the configuration block, and the error taxonomy (connectivity,
  constraint classes, version mismatch). Built. In v0.4 (planned) the seam maps every error,
  `Session` becomes the stdlib method set with `PrepareContext`, and `Transact[T]` with
  options is the one transaction runner.
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
  postgres dialect — `$N` placeholders, class-23 constraint classification, the `RETURNING`
  capability. Built; v0.4 drops the `ast` capabilities and adds the migrate lock.
- **query** — planned: the mechanism over authored SQL files — statements with named
  parameters, the `Source` inventory, typed handles bound once at wiring, the projection with
  its declared field contract and directives, the guard, prepare-based verification, and the
  protocol-only pattern templates. See `concepts/sql-architecture.md`.
- **migrate** — planned: schema versioning over embedded SQL — the history table, per-file
  transactions with a header opt-out, dirty and force semantics, the lock capability. See
  `concepts/sql-architecture.md`.
- **internal/drivertest** — planned: the one shared scripted driver with prepare support,
  replacing the fakes duplicated across four packages.

`design/layers.md` describes v0.3.0 as built and is superseded by the DSL strategy. The v0.4
direction is `concepts/sql-architecture.md`, settled by evidence from the
`v1.data.sql.prototype` experiment and rewritten into design by the sessions that extract it.
