# go-database

go-database is an SQL infrastructure library for Standards Lab's Go Elemental standard. It
holds four things:

- the connection pool, with its configuration, lifecycle, and readiness
- the database admin service, built over the `sqlate` library
- the sweeper, which runs a service's reclaimers
- the Postgres provider
- the pgvector module, which resolves a configured distance metric

`github.com/standards-lab/go-database` is the base module, holding the `database` package at its
root and `admin` and `sweep` beside it. `github.com/standards-lab/go-database/postgres` is a
nested sub-module that pins the driver and is released on its own tags.
`github.com/standards-lab/go-database/postgres/pgvector` is a nested sub-module that requires
sqlate's `postgres/pgvector` and is released on its own tags.

## Standard

`go-database` is an infrastructure library of
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md), the
minimal-dependency Go standard. This README and each package's `doc.go` document the
repository; the standard's principles it enhances are stated below. Its repository-level
principles:

- The base module depends on the standard library, `go-core`, and `sqlate`; the driver enters a
  consumer's graph only through the `postgres` sub-module, imported once at the composition root.
- The `postgres/pgvector` sub-module depends on the standard library and sqlate's
  `postgres/pgvector`. Importing it is limited the way a provider's is: the composition root, the
  migrations, and the packages the application's design documentation declares.
- [`sqlate`](https://github.com/standards-lab/sqlate) owns the SQL layer above the pool:
  statements, sessions, transactions, and the dialect. The composition root wraps the pool this
  library constructs with `sqlate.Wrap` and the engine's dialect from `sqlate/postgres`, and the
  provider's native API stays reachable through `DB.Conn()` and the `Options` map.
- Swapping the SQL provider is a port. The consumer owns its schema, its migrations, and its
  domain SQL, and those are what a swap rewrites. A library that ships its own migration set,
  such as `blobfs`, owns that set; the consumer declares it beneath its own.
- The HTTP half of the admin service, the route group and handler, is application code; this
  library ships none.

## Packages

- `database` wraps a `database/sql` pool with lifecycle integration and holds the service's
  configuration block and its connectivity error sentinels.
- `admin` is the database admin service: schema verification, migration, seeding, named states,
  and diagnostics, run at startup and on demand as operations over `sqlate`'s functions. One
  service administers every migration set its migrator runs: status and reset cover them all,
  and the verbs that target one set, including `force` for dirty-set repair, take its name.
- `sweep` runs a service's reclaimers, the bounded passes its libraries' garbage is removed in,
  as one go-core reactor: each on its own interval or when nudged by name, under a per-reclaimer
  advisory lock so one replica reclaims at a time, and paused under the service's quiesce gate.
  It imports no reclaimer; a library's reclaimer satisfies its interface.
- `postgres` is the PostgreSQL provider: it constructs the pool over pgx's `database/sql` adapter
  from the configuration block.
- `pgvector` resolves the pgvector distance metric an application's configuration names into
  sqlate's metric table, and its package doc shows pgvector-go's vector types used directly.

## Development

The repository uses a Go workspace and [mise](https://mise.jdx.dev):

```
mise run check      # build, vet, format, fix, tidy, test, and lint every module, then lint-context and lint-references; writes nothing
mise run currency   # report requirements, Go, tools, and actions behind their latest
mise run upgrade    # upgrade every module's go directive and requirements, and the tools, to their latest
```

## License

[Apache License 2.0](LICENSE).
