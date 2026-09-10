# go-database

go-database is the SQL infrastructure library of Go Elemental, the Standards Lab
organization's Go implementation of the Elemental Architecture. It holds the connection pool
with its configuration, lifecycle, and readiness; the database admin service over the `sqlate`
library; and the driver, isolated in the `postgres` sub-module. The repository is managed with
the marathon workflow; start from `context/README.md`.

## Documentation lives in the repository

This repository documents its own implementation: the README states its place in the standard
and the principles it enhances, and each package's `doc.go` is the authority for its API. The
organization's [architecture repository](https://github.com/standards-lab/architecture) states the principles this repository follows
and holds nothing a reader can infer from this source. `context/` records only working
knowledge the code and the README do not express; do not restate documented design here. A
change that alters documented behavior updates the README and the package documentation in
the same effort, and a design note that generalizes past this repository is promoted to the
architecture repository through its `context/`.

## Repository specifics

- **Module layout.** One base module is rooted at `github.com/standards-lab/go-database`,
  holding the `database` package at its root and `admin` beside it. Provider sub-modules carry
  their own `go.mod`, each named for its target system — `postgres` today. A provider
  constructs the pool and supplies no dialect; the dialect is `sqlate/postgres`'s.
- **Local development.** Development uses the committed root `go.work`. Pinned `require`
  versions are the committed steady state; a `replace` directive is only a transient bridge
  while a provider builds against unreleased base changes.
- **Standard conformance.** Dependencies, releases, CI, tests, and tasks follow the Go Elemental
  standard principles in the architecture repository (base `v*` and `postgres/v*` tags,
  per-module CI matrix, hermetic driver-stub tests, mise tasks looping over the modules).
- **Public repo.** Modules resolve through the public Go proxy; CI carries no private-module
  config.
