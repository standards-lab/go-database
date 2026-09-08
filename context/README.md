# go-database

The SQL infrastructure library of Go Elemental, the Standards Lab organization's Go
implementation of the Elemental Architecture: the connection pool with its configuration,
lifecycle, and readiness, the database admin service over the `sqlate` library, and the driver
isolated in the `postgres` provider sub-module.

The README and each package's `doc.go` document this repository, and the
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md) standard states the principles it follows; this context records only
working knowledge the code and the README do not express.

## Capability map

The built packages are authoritative through their code and `doc.go`. Detail for what is unbuilt
is added when it is about to be built.

- **database**: the infrastructure service over the pool: configuration, lifecycle,
  readiness, and the connectivity sentinels. Built.
- **admin**: the database admin service over `sqlate`, generic over the consumer's migrator,
  seeder, registry, catalog, and pool; its startup sequence, its verbs, named states, and the
  destructive class are `admin/doc.go`. One migration set per service: a service administering
  several schemas registers several services under distinct lifecycle names. Built; named
  states landed at `v1.data.sql.tasks.states` (2026-09-07).
- **postgres**: the PostgreSQL provider: the pool over pgx's `database/sql` adapter, constructed
  from the configuration block. It supplies no dialect; the dialect is `sqlate/postgres`'s.
  Built.

Statements, sessions, transactions, the dialect, migrations, and the scripted test driver are
`sqlate`'s (`github.com/standards-lab/sqlate`), a standalone library below this one, as the
`database` package comment states. The HTTP half of the admin service, the route group and
handler, is application code; the reference service built it at
`v1.data.sql.integration.service` (2026-09-06), and the template stays engine-free.
