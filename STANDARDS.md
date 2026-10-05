# go-database standards

The judgement calls the standards-reviewer applies to go-database.

- A change that alters documented behavior updates the README and the affected `doc.go` in the same change.
- `architecture/standards/go-elemental/principles/dependencies.md`: the bottom-up line and no provider in a base, held by review, across the base module's `go.mod` and `postgres/go.mod`, the one module that pins the driver.
- `architecture/standards/go-elemental/principles/tests-and-docs.md`: the doc.go inventory of `database`, `admin` and `postgres`, held by review, and the unit tier over sqlate's `sqltest`, the `database` tests' stub driver and the `postgres` tests' in-test `pgproto3` server.
- `architecture/standards/go-elemental/principles/topology-and-naming.md`: the base module holding `database` at its root and `admin` beside it, and the `postgres` provider sub-module and its `postgres/v*` tags.
- `architecture/standards/go-elemental/principles/release-and-ci.md`: the check, currency and upgrade tasks over `GO_MODULES`, and a changelog per module, `CHANGELOG.md` and `postgres/CHANGELOG.md`.
- `architecture/standards/go-elemental/principles/baseline-standards.md`: `Config`'s pool-size and timeout defaults, which the application's configuration overrides, and `admin`, which takes every migration set, seed set, catalog and registry from its consumer.
- `architecture/principles/service-tiers.md`: `DB.Conn` and `Config.Options` as the native tier beneath the pool, and the `postgres` provider, which constructs the pool and supplies no dialect.
- `architecture/principles/validation-first.md`: `Config.Finalize` validates before `New` accepts the config, and `admin`'s verbs refuse an argument outside their domain before any I/O.
- `architecture/principles/context-architecture.md`: the README and each `doc.go` are the homes; `context/` records only what they do not express.
