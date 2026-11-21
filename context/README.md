# go-database

go-database is the SQL infrastructure library of Go Elemental, the Standards Lab
organization's Go implementation of the Elemental Architecture. It holds the connection pool
with its configuration, lifecycle, and readiness; the database admin service over the `sqlate`
library; the general sweeper; and the driver, isolated in the `postgres` provider sub-module.

The README and each package's `doc.go` document this repository. The
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md) standard states the principles it follows.
This context records only working knowledge the code and the README do not express.

## Capability map

- **database** (built): wraps the pool: configuration, lifecycle, readiness, and the
  connectivity sentinels ([`doc.go`](../doc.go)).
- **admin** (built): the database admin service over `sqlate`, generic over the consumer's
  migrator, seeder, registry, catalog, and pool. [`admin/doc.go`](../admin/doc.go) states its
  construction, its verbs, its startup sequence, its errors, and its migration sets: one service
  administers every set its migrator runs.
- **sweep** (built): the general sweeper: it runs a service's reclaimers as one go-core reactor,
  each cycle under the advisory lock `sweep.<name>` and bounded by its interval.
  [`sweep/doc.go`](../sweep/doc.go) states its construction, cycles, and drain.
- **postgres** (built): the PostgreSQL provider: it constructs the pool over pgx's
  `database/sql` adapter from the configuration block, and supplies no dialect — the dialect is
  `sqlate/postgres`'s ([`postgres/doc.go`](../postgres/doc.go)).
