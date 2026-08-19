# Releases and CI

How the modules are versioned, released, built, and tested.

## Releases: the base module and each provider

There are two kinds of releasable artifact. The base module is released by pushing a root tag
`v<semver>`; that one version covers `database` and `seed`. Each provider sub-module is released by
pushing a tag `<path>/v<semver>` (`postgres/v0.1.0`). There is no umbrella version spanning the base
and the providers, so a consumer pulling one provider is not entangled with another's version.

`.github/workflows/release.yml` derives the artifact from the tag: a bare `v*` tag reads the root
`CHANGELOG.md`; a `<path>/v*` tag reads that sub-module's `CHANGELOG.md`. It cuts a GitHub release
with `taiki-e/create-gh-release-action`. Each artifact keeps its own `CHANGELOG.md` in
Keep-a-Changelog form with dated headings (`## [vX.Y.Z] - YYYY-MM-DD`), so the release action slices
the right section.

Between releases, a prerelease tag (`v0.2.0-dev.1`, `postgres/v0.2.0-dev.1`) lets a consumer
resolve an API still in flux. A minor-or-above release purges them: its changelog entry states
that the release includes every `vX.Y.Z-dev.N` change, and those tags are removed.

## Branches and tags

Release preparation commits directly to `main`: the changelog date, the license, and README or
metadata edits. Tagging a commit on a session branch would start the release workflow before `main`
contained that commit. The tag is pushed only after main's CI run passes, so a release never points
at a commit that failed CI.

No release branch is retained. The tag is the durable artifact, and a branch can be recreated from
its tag at any time (`git switch -c release/v0.1.x v0.1.0`). Create a `release/v<major>.<minor>.x`
branch only when a maintenance line exists, meaning a patch to an older minor after `main` has moved
past it, and keep it only as long as that line is supported. Every other branch is deleted when it
merges; the repository has `deleteBranchOnMerge` enabled.

A published tag is not re-cut. Once the module proxy has fetched a version, the checksum database
pins that commit permanently, so moving the tag leaves consumers with a mismatch they cannot resolve.
A failed release workflow does not affect module resolution, because `go get` reads the tag rather
than the GitHub release: fix the workflow on `main` and create the release for the existing tag with
`gh release create`.

## What ships in the module zip

`context/`, `CLAUDE.md`, and `.claude/` are tracked files, so they appear in the module zip consumers
download. Go provides no supported way to exclude them, and they cost nothing: the toolchain compiles
only imported `.go` files. Left as is.

## Coordinating a change across the base and a provider

`go.work` (committed, at the repository root) resolves the base module and the provider sub-modules
during local development, so a change spanning them is built and tested together before anything is
tagged.

A provider depends on the base through a pinned `require`. Pinned versions are the committed steady
state; a `replace` directive is only a transient bridge while the base carries unreleased changes a
provider needs, removed once the base is tagged. The release ripple runs bottom-up across the
base→provider edge: tag the base, bump the provider's `require`, note it in the provider's changelog,
tag the provider.

## CI

`.github/workflows/ci.yml` runs a per-module matrix: `go vet`, then `go test -race`, then
golangci-lint, each with `working-directory` set to the module. The repository is public, so modules
resolve through the normal Go proxy and checksum database; CI carries no `GOPRIVATE` or `.netrc`
configuration. The tests need no database container (`tests-and-docs.md`).

The CI matrix, the `go.work` use-list, and `mise`'s `GO_MODULES` are three lists that name the same
modules — the base module (`.`) and each provider sub-module — and are updated together whenever a
module is added. Each matrix entry's steps are guarded on the module's `go.mod`, so an entry stays
green until that module lands. The import boundary that confines provider imports is a consumer-side
check and runs in consumers, not here; inside this repository the module topology already keeps the
base from importing a provider.

## Tasks

`mise.toml` defines the developer tasks — `build`, `test`, `vet`, `fmt`, `tidy`, `lint` — each
looping over `GO_MODULES`. `mise run test` builds and tests the base module and every provider
sub-module.
