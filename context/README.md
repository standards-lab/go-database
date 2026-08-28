# go-database

The SQL infrastructure library of Go Minimal, the Standards Lab organization's minimal-dependency
Go standard: the database service in four layers, with the driver isolated in the `postgres`
provider sub-module.

The design and conventions of this repository are documented in the organization's
[documentation landing zone](https://github.com/standards-lab/docs); this context records only
working knowledge the landing zone and the code do not express. The repository page is
[go-database](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/index.md),
under the [Go Minimal](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/index.md)
standard, with the design detailed in
[Service tiers in SQL](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/tiers.md),
[The layer ontology](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/layers.md),
[The dialect interface](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/dialect.md),
and
[Providers](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/providers.md).

## Capability map

The built packages are authoritative through their code and `doc.go`; the landing zone documents
the design. Detail for what is unbuilt is added when it is about to be built.

- **database** — the service layer: the wrapper, the `Dialect` interface, the `Session`/`Tx`
  seam with `ExecTx`, the configuration block, and the error taxonomy (connectivity,
  constraint classes, version mismatch). Built.
- **ast** — the statement layer: standard SQL as values — expressions, predicates, tables,
  `Select`/`Compound` as the sealed `Query` expressions, `Insert`/`Update`/`Delete` as the
  write statements — rendered to `SQL` values through the dialect, with the two-direction
  capability pattern (`PagingRenderer` override, `ReturningRenderer` declared-native). Built.
- **operation** — the contract layer: `Projection` with the `List` and `One` queries and the
  `Directives` request vocabulary; `Insertion` and the guarded commands on the
  optimistic-concurrency contract; the shared `Field` vocabulary and the `RecursivePath`
  computed-field pattern. Built.
- **exec** — the execution layer: `Scan`, the query runners over `Session`, the command
  runners over `*Tx` with guard-outcome mapping, every driver error through `MapError`. Built.
- **seed** — reference-data loading: a runner over the consumer's seed file system, one
  transaction per step, the decode format selected by extension. Built.
- **postgres** — the PostgreSQL provider: the pool over pgx's `database/sql` adapter and the
  postgres dialect — `$N` placeholders, class-23 constraint classification, the `RETURNING`
  capability. Built.

The settled intent behind the layers — the ontology, the capability taxonomy and divergence
ledger, the guard contract, the held seams — is `design/layers.md`.
