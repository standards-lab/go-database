# go-database

go-database is the SQL infrastructure library of Go Elemental, the Standards Lab
organization's Go implementation of the Elemental Architecture. It holds the connection pool
with its configuration, lifecycle, and readiness; the database admin service over the `sqlate`
library; the general sweeper; the driver, isolated in the `postgres` provider sub-module; and the
`postgres/pgvector` sub-module, which resolves a configured pgvector distance metric.

The README and each package's `doc.go` document this repository. The
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md) standard states the principles it follows.

## Capability map

- **database** (built): the connection pool, with its configuration, lifecycle, readiness, and
  connectivity sentinels ([doc.go](../doc.go)).
- **admin** (built): the database admin service over `sqlate`, which administers every migration
  set its migrator runs and reports every extension declaration in its status and readiness
  ([admin/doc.go](../admin/doc.go)).
- **sweep** (built): the general sweeper that runs a service's reclaimers
  ([sweep/doc.go](../sweep/doc.go)).
- **postgres** (built): the PostgreSQL provider, which constructs the pool over pgx's
  `database/sql` adapter ([postgres/doc.go](../postgres/doc.go)).
- **postgres/pgvector** (built): the sub-module whose `ParseMetric` resolves a configured
  distance metric into sqlate's metric table, and whose package doc shows pgvector-go used
  directly ([postgres/pgvector/doc.go](../postgres/pgvector/doc.go)).
- **integration tier** (built): a compose PostgreSQL with pgvector, and the live tests behind the
  `integration` tag that run against it locally through `mise run acceptance`
  ([README](../README.md#development)).
