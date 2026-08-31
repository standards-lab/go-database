# go-database

SQL infrastructure library for Standards Lab's Go Elemental standard: the data layer's standard
tier, reference-data seeding, and the Postgres provider.

`github.com/standards-lab/go-database` is the base module, holding the `database` package at its
root and `seed` beside it. `github.com/standards-lab/go-database/postgres` is a nested sub-module
that pins the driver and is released on its own tags.

## Standard

`go-database` is an infrastructure library of
[Go Elemental](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/index.md), the
minimal-dependency Go standard, and its design is documented on the standard's
[go-database page](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/index.md).
Its repository-level principles:

- The base module depends on the standard library and `go-core`; the driver enters a consumer's
  graph only through the `postgres` sub-module, imported once at the composition root.
- The `database` package is the standard tier, ISO/IEC 9075 SQL; the provider's native API stays
  reachable through `DB.Conn()` and the `Options` map.
- Swapping the SQL provider is a port, not a configuration change: the consumer owns its schema,
  its migrations, and its domain SQL, and those are what a swap rewrites.

## Packages

- `database` — the dialect-neutral core: a lifecycle-integrated wrapper over a `database/sql` pool,
  the `Dialect` interface providers implement, the service's configuration block, and the error
  sentinels.
- `seed` — loads reference data from seed files, one transaction per step, with the decode format
  selected by file extension.
- `postgres` — the PostgreSQL provider: constructs the pool over pgx's `database/sql` adapter and
  supplies the postgres dialect.

## Development

The repository uses a Go workspace and [mise](https://mise.jdx.dev):

```
mise run test    # build and test every module
```

## License

[Apache License 2.0](LICENSE).
