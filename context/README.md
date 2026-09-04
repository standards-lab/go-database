# go-database

The SQL infrastructure library of Go Elemental, the Standards Lab organization's Go
implementation of the Elemental Architecture: the connection pool with its configuration,
lifecycle, and readiness, the database admin service over the `sqlate` library, and the driver
isolated in the `postgres` provider sub-module.

The design and conventions of this repository are documented in the organization's
[documentation landing zone](https://github.com/standards-lab/docs); this context records only
working knowledge the landing zone and the code do not express. The repository page is
[go-database](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/go-database/index.md),
under the [Go Elemental](https://github.com/standards-lab/docs/blob/main/standards/go-elemental/index.md)
standard. Its pages `layers.md`, `dialect.md`, and `index.md` describe v0.3.0 and are rewritten
by the docs pass (`v1.data.sql.tasks.docs`); until then `design/infrastructure-service.md` is the
design record for v0.4.0.

## Capability map

The built packages are authoritative through their code and `doc.go`. Detail for what is unbuilt
is added when it is about to be built.

- **database**: the infrastructure service: the wrapper over the pool with `Start`, `Shutdown`,
  `Ready`, and `Ping`; the configuration block with its environment overrides; and the two
  connectivity sentinels. Built, v0.4.0.
- **admin**: the database admin service over `sqlate`: `Start` verifies the migration history,
  applies a pending set, verifies the seeder, and seeds when enabled; the verbs, `Seed`,
  `Diagnose`, `Catalog`, and `Statements` are triggers over the library's functions. It is
  generic over a prebuilt migrator, a `Seeder`, a `Registry`, the pattern catalog, and the pool.
  Built, v0.4.0.
- **postgres**: the PostgreSQL provider: the pool over pgx's `database/sql` adapter, constructed
  from the configuration block. It supplies no dialect; the dialect is `sqlate/postgres`'s. Built,
  postgres/v0.3.0.

Statements, sessions, transactions, the dialect, migrations, and the scripted test driver are
`sqlate`'s (`github.com/standards-lab/sqlate`, v0.1.0), a standalone library below this one. The
HTTP half of the admin service, the route group and handler, is application code the template
scaffolds (`v1.data.sql.integration.template`).
