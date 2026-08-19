# go-database

The database capability of `go-minimal`, the Standards Lab organization's minimal-dependency Go
standard: the SQL data layer's standard tier in the base module, with the provider's native API
reached through the handle it exposes and the driver isolated in the `postgres` sub-module. Managed
with the marathon workflow; start from `context/README.md`.

## Conventions are settled in the repository

The design and conventions for this capability are recorded in `context/design/` — that is the
authority. Keep them there; do not restate them here.

## Role boundary

go-database is a marathon **code** project (`.claude/marathon.toml` declares `kind = "code"`). The
developer owns the production Go source — they apply it and answer for it. The agent writes
everything else: tests, godoc and `doc.go`, prose documentation, the files in `context/`, the
implementation guide, and the reset file.

## Repository specifics

- **Module layout** — one base module rooted at `github.com/standards-lab/go-database`, holding the
  `database` package at its root and `seed` beside it, plus provider sub-modules with their own
  `go.mod`, each named for the target system: `postgres` today.
- **Dependencies** — the base module depends on the standard library and `go-core`; at most,
  packages as idiomatic and stable as the standard library. A driver enters only through its
  provider sub-module.
- **Local development** uses the committed root `go.work`; pinned `require` versions are the
  committed steady state. A `replace` directive is only a transient bridge while the base carries
  unreleased changes a provider needs, removed when the base is tagged.
- **Releases** — the base module is tagged `v<semver>` at the root from `CHANGELOG.md`; each
  provider sub-module is tagged `<path>/v<semver>` from its own `CHANGELOG.md`, cut by
  `.github/workflows/release.yml`.
- **Tests** are co-located `{file}_test.go` files in an external black-box package
  (`package <pkg>_test`) that exercise the public API. They are hermetic: driver stubs stand in
  for an engine, and no test dials a database.
- **Tasks** run through `mise` (`build`, `test`, `vet`, `fmt`, `tidy`, `lint`), each looping over
  the modules.
- **Public repo.** Modules resolve through the public Go proxy; CI carries no private-module config.
