# go-database

The SQL infrastructure library of Go Minimal, the Standards Lab organization's minimal-dependency
Go standard: the database service in two tiers, with the driver isolated in the `postgres`
provider sub-module.

The design and conventions of this repository are documented in the organization's
[documentation landing zone](https://github.com/standards-lab/docs); this context records only
working knowledge the landing zone and the code do not express. The repository page is
[go-database](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/index.md),
under the [Go Minimal](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/index.md)
standard, with the design detailed in
[Service tiers in SQL](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/tiers.md),
[The dialect interface](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/dialect.md),
and
[Providers](https://github.com/standards-lab/docs/blob/main/standards/go-minimal/go-database/providers.md).

## Capability map

The built packages are authoritative through their code and `doc.go`; the landing zone documents
the design. Detail for what is unbuilt is added when it is about to be built.

- **database** — the dialect-neutral core: the wrapper, the `Dialect` interface, the
  configuration block, and the error sentinels. Built.
- **seed** — reference-data loading: a runner over the consumer's seed file system, one
  transaction per step, the decode format selected by extension. Built.
- **postgres** — the PostgreSQL provider: the pool over pgx's `database/sql` adapter and the
  postgres dialect. Built.
- **Candidate direction** — the query vocabulary (`concepts/query.md`) and the write path
  (`concepts/writes.md`). Each begins with an evaluation of idiomatic patterns across the common
  engines, settling the standard and native sets and what `Dialect` grows to hold, and each is
  built in its own session.
