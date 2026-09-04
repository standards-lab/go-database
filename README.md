# go-database

SQL infrastructure library for Standards Lab's Go Elemental standard: the connection pool with
its configuration, lifecycle, and readiness, the database admin service over the `sqlate`
library, and the Postgres provider.

`github.com/standards-lab/go-database` is the base module, holding the `database` package at its
root and `admin` beside it. `github.com/standards-lab/go-database/postgres` is a nested sub-module
that pins the driver and is released on its own tags.

## Standard

`go-database` is an infrastructure library of
[Go Elemental](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/index.md), the
minimal-dependency Go standard, and its design is documented on the standard's
[go-database page](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/index.md).
Its repository-level principles:

- The base module depends on the standard library, `go-core`, and `sqlate`; the driver enters a
  consumer's graph only through the `postgres` sub-module, imported once at the composition root.
- Statements, sessions, transactions, and the dialect are
  [`sqlate`](https://github.com/standards-lab/sqlate)'s: the composition root wraps the pool this
  library constructs with `sqlate.Wrap` and the engine's dialect from `sqlate/postgres`. The
  provider's native API stays reachable through `DB.Conn()` and the `Options` map.
- Swapping the SQL provider is a port, not a configuration change: the consumer owns its schema,
  its migrations, and its domain SQL, and those are what a swap rewrites.

## Packages

- `database`: the infrastructure service, a lifecycle-integrated wrapper over a `database/sql`
  pool, the service's configuration block, and the connectivity error sentinels.
- `admin`: the database admin service, schema verification, migration, seeding, and
  diagnostics as operations over `sqlate`'s functions, run at startup and on demand.
- `postgres`: the PostgreSQL provider, which constructs the pool over pgx's `database/sql` adapter from
  the configuration block.

## Development

The repository uses a Go workspace and [mise](https://mise.jdx.dev):

```
mise run build   # build every module standalone, with the workspace off
mise run test    # test every module
```

## License

[Apache License 2.0](LICENSE).
