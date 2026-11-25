# go-database

go-database is an SQL infrastructure library for Standards Lab's Go Elemental standard. It
holds five things:

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
  `postgres/pgvector`. Only its integration tier requires pgvector-go, the base module, and
  sqlate's `postgres`, so its `go.mod` lists them as test-only requirements. Like a provider, it
  is imported only by the composition root, the migrations, and the packages the application's
  design documentation declares.
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
mise run currency   # report tools, Go, requirements, actions, images, and Dockerfile source ARGs behind their latest, except a held one; writes nothing
mise run upgrade    # move every pin currency reports to its latest, except a held one: tools, then Go and requirements, actions, images, and Dockerfile source ARGs
mise run db:up      # build and start the compose PostgreSQL and wait until it is healthy
mise run db:down    # stop PostgreSQL, discarding its data
mise run integration  # run the integration tier against the running stack
mise run acceptance   # db:up, integration, then db:down whatever the result; prints the stack's log on failure
```

`compose.yml` runs PostgreSQL with pgvector on port 5438; `POSTGRES_PORT` moves it, and
`GO_DATABASE_DSN` in `mise.toml` follows it. The service builds from
`compose/postgres/Dockerfile`, a copy of sqlate's canonical PostgreSQL image: its `FROM` line is
the one image pin, `ARG PGVECTOR_VERSION` the extension's, and `mise run currency` reports either
when it trails its latest; `mise run upgrade` moves it. The two tasks run `scripts/currency.sh`'s
`report` and `upgrade` modes, which run the same scans, so `upgrade` moves exactly what `report`
lists. A pin named in `mise.toml`'s `HOLDS` list, one
`<item> <pin>: <reason>` line each, is reported as held and never moved. The stack is a test-only
harness that keeps its data on tmpfs, so every `db:up` starts empty.

The `postgres/pgvector` module's integration tier sits behind the `integration` build tag and
runs against the stack: `admin.Start` activating a declared `vector` extension, a `vector`
column round-tripping pgvector-go values and NULL, and each metric's operator ordering a
nearest-neighbor query. Each test makes its own throwaway database through sqlate's
`postgres/pgtest`. The tier runs on a developer's machine only, through `mise run acceptance`,
and needs Docker; CI runs `mise run check`, which vets, fixes, and lints the tier with its tag
but runs only the unit tier.

## License

[Apache License 2.0](LICENSE).
