# go-database standards

The judgement calls the standards-reviewer applies to go-database, beyond what `mise run check` enforces.

- A change that alters documented behavior updates the README and the affected `doc.go` in the same change.
- `architecture/standards/go-elemental/principles/dependencies.md`: the bottom-up line and no provider in a base, across the base module's and `postgres`'s `go.mod`.
- `architecture/standards/go-elemental/principles/tests-and-docs.md`: the doc.go inventory of `database`, `admin` and `postgres`, and the unit tier over sqlate's `sqltest`, the `database` tests' stub driver and the `postgres` tests' in-test `pgproto3` server.
- `architecture/standards/go-elemental/principles/topology-and-naming.md`: the base module with `database` and `admin`, and the `postgres` provider sub-module and its tags.
- `architecture/standards/go-elemental/principles/release-and-ci.md`: the check, currency and upgrade tasks over `GO_MODULES`, and a changelog per module, `CHANGELOG.md` and `postgres/CHANGELOG.md`.
- `architecture/standards/go-elemental/principles/lifecycle-and-context.md`: `DB`'s `Start`, `Shutdown` and `Ready`, and `admin.Service`'s `Start` and `Ready`; the constructors' panics on an unfinalized `Config` or a missing pool, and `postgres.New`'s error on a reserved option.
- `architecture/standards/go-elemental/principles/baseline-standards.md`: `Config`'s pool-size and timeout defaults, and `admin`, which takes every migration set, seed set, catalog and registry from its consumer.
- `architecture/principles/service-tiers.md`: `DB.Conn`, `Config.Options`, and the `postgres` provider.
- `architecture/principles/validation-first.md`: `Config.Finalize`, and `admin`'s verbs, which refuse an argument outside their domain before any I/O.
- `architecture/principles/context-architecture.md`: the README and each `doc.go` are the homes.
