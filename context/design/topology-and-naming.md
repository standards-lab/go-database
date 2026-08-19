# Topology and naming

How the capability's modules are organized and named.

## One base module, providers as modules of their own

The base module is one Go module rooted at the repository: the `database` package at its root and
`seed` beside it, versioned and released together as one artifact. A package the capability gains
later, such as the query vocabulary, is another package in the base module.

A provider is a module of its own that pins its engine's driver, so a consumer takes the base and
one provider without pulling any other driver; the base module carries none (`dependencies.md`). The
providers this repository answers for are nested sub-modules with their own `go.mod`, versioned and
released on their own schedule: `postgres`. A provider for another engine may be a nested sub-module
here or an external module in another repository; the contract it implements is the same
(`providers.md`).

Keeping the base and its nested providers in one repository makes a change that spans them atomic:
it is made and exercised together through `go.work` before any tag is cut.

## Naming

- **Repository and base module:** `github.com/standards-lab/go-database`. The package at the module
  root keeps its own short name, `database`, so the import path carries the tier and the identifier
  in code stays the package's: `database.New`, `database.Config`. `seed` is
  `github.com/standards-lab/go-database/seed`.
- **Provider sub-modules** are nested directories named for the engine, never the driver:
  `postgres`. Naming by engine lets the driver change without renaming the module. Each
  sub-module's path is its directory appended to the base:
  `github.com/standards-lab/go-database/postgres`.
- **Release tags:** the base module is tagged `v<semver>` at the repository root; each provider
  sub-module is tagged with its directory as prefix, `<path>/v<semver>` (`postgres/v0.1.0`).

## The cost and the benefit

The cost is a changelog and a tag namespace per released artifact: the base plus each nested
provider. The benefit is that the base moves as one version while each provider stays independently
consumable. The cost is bounded by how providers are added: one nested provider by default, and a
provider for another engine where that engine is needed.
