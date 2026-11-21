# go-database

go-database is the SQL infrastructure library of Go Elemental, the Standards Lab
organization's Go implementation of the Elemental Architecture. It holds the connection pool
with its configuration, lifecycle, and readiness; the database admin service over the `sqlate`
library; the general sweeper; and the driver, isolated in the `postgres` provider sub-module.

The README and each package's `doc.go` document this repository. The
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md) standard states the principles it follows.

## Capability map

- **database** (built): the connection pool, with its configuration, lifecycle, readiness, and
  connectivity sentinels ([doc.go](../doc.go)).
- **admin** (built): the database admin service over `sqlate`, which administers every migration
  set its migrator runs ([admin/doc.go](../admin/doc.go)).
- **sweep** (built): the general sweeper that runs a service's reclaimers
  ([sweep/doc.go](../sweep/doc.go)).
- **postgres** (built): the PostgreSQL provider, which constructs the pool over pgx's
  `database/sql` adapter ([postgres/doc.go](../postgres/doc.go)).
