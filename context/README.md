# go-database

go-database is the SQL infrastructure library of Go Elemental, the Standards Lab
organization's Go implementation of the Elemental Architecture. It holds the connection pool
with its configuration, lifecycle, and readiness; the database admin service over the `sqlate`
library; the general sweeper; and the driver, isolated in the `postgres` provider sub-module.

The README and each package's `doc.go` document this repository. The
[Go Elemental](https://github.com/standards-lab/architecture/blob/main/standards/go-elemental/README.md) standard states the principles it follows.
This context records only working knowledge the code and the README do not express.

## Capability map

- **database** (built): [doc.go](../doc.go).
- **admin** (built): [admin/doc.go](../admin/doc.go).
- **sweep** (built): [sweep/doc.go](../sweep/doc.go).
- **postgres** (built): [postgres/doc.go](../postgres/doc.go).
