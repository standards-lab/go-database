# go-database

go-database is an SQL infrastructure library for Standards Lab's Go Elemental standard. It
holds three things:

- the connection pool, with its configuration, lifecycle, and readiness
- the database admin service, built over the `sqlate` library
- the Postgres provider

`github.com/standards-lab/go-database` is the base module, holding the `database` package at its
root and `admin` beside it. `github.com/standards-lab/go-database/postgres` is a nested sub-module
that pins the driver and is released on its own tags.

## Standard

`go-database` is an infrastructure library of
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md), the
minimal-dependency Go standard. This README and each package's `doc.go` document the
repository; the standard's principles it enhances are stated below. Its repository-level
principles:

- The base module depends on the standard library, `go-core`, and `sqlate`; the driver enters a
  consumer's graph only through the `postgres` sub-module, imported once at the composition root.
- [`sqlate`](https://github.com/standards-lab/sqlate) owns the SQL layer above the pool:
  statements, sessions, transactions, and the dialect. The composition root wraps the pool this
  library constructs with `sqlate.Wrap` and the engine's dialect from `sqlate/postgres`, and the
  provider's native API stays reachable through `DB.Conn()` and the `Options` map.
- Swapping the SQL provider is a port. The consumer owns its schema, its migrations, and its
  domain SQL, and those are what a swap rewrites.

## Packages

- `database` wraps a `database/sql` pool with lifecycle integration and holds the service's
  configuration block and its connectivity error sentinels.
- `admin` is the database admin service: schema verification, migration, seeding, named states,
  and diagnostics, run at startup and on demand as operations over `sqlate`'s functions.
- `postgres` is the PostgreSQL provider: it constructs the pool over pgx's `database/sql` adapter
  from the configuration block.

## Development

The repository uses a Go workspace and [mise](https://mise.jdx.dev):

```
mise run build   # build every module standalone, with the workspace off
mise run test    # test every module
```

## License

[Apache License 2.0](LICENSE).
