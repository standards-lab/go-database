# go-database

The SQL infrastructure library of Go Minimal, the Standards Lab organization's minimal-dependency
Go standard: the database service's standard tier in the base module, the provider's native API
reached through the handle it exposes, and the driver isolated in the `postgres` sub-module.
Managed with the marathon workflow; start from `context/README.md`.

## Design is documented in the landing zone

The design and conventions of this repository are documented in the organization's
[documentation landing zone](https://github.com/standards-lab/docs) — that is the authority.
`context/` records only working knowledge the landing zone and the code do not express; do not
restate documented design here. A change that alters documented behavior updates the landing
zone page in the same effort.

## Role boundary

go-database is a marathon **code** project (`.claude/marathon.toml` declares `kind = "code"`).
The developer owns the production Go source — they apply it and answer for it. The agent writes
everything else: tests, godoc and `doc.go`, prose documentation, the files in `context/`, the
implementation guide, and the reset file.

## Repository specifics

- **Module layout** — one base module rooted at `github.com/standards-lab/go-database`, with the
  `database` package at its root and `seed` beside it, plus provider sub-modules with their own
  `go.mod`, each named for the target system: `postgres` today.
- **Local development** uses the committed root `go.work`; pinned `require` versions are the
  committed steady state, and a `replace` directive is only a transient bridge while a provider
  builds against unreleased base changes.
- **Dependencies, releases, CI, tests, tasks** — per the Go Minimal standard principles in the
  landing zone (base `v*` and `postgres/v*` tags, per-module CI matrix, hermetic driver-stub
  tests, mise tasks looping over the modules).
- **Public repo.** Modules resolve through the public Go proxy; CI carries no private-module
  config.
