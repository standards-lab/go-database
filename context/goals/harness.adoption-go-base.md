# goal · harness.adoption-go-base

- **State:** building
- **Task:** adopt
- **Branch:** adopt

## Tasks

1. [ ] adopt

## Task brief · adopt

**Problem**

go-database, go-observability, and go-cli-sdk predate marathon 0.22.0. Nothing pins marathon's
tools, the check doesn't run lint-context or lint-references, and every capability map fails
lint-context. Hundreds of comments fall outside the five kinds, and no lint requires a
suppression's reason.

**Behaviors**
1. Each repository's currency command exits 0 after its upgrade slice. GitHub Actions use
   mise-action v5.2.10.
2. Each `mise.toml` pins marathon's tools at 0.22.0 with the `marathon/v` prefix. `mise install`
   puts lint-context and lint-references on PATH, and currency reports a newer marathon tag.
3. Each check runs lint-context and lint-references once, after its module loop, and fails when
   either fails. A lint missing from PATH fails the check with a message naming `mise install`.
   The task description names both lints.
4. lint-context passes in each repository:
   - Every capability map entry is marked `(built)` or `(planned)`, with no prose under the map
     heading.
   - go-observability's logs pipeline is a linked `(planned)` entry whose note holds its open
     points, with the trigger as the README's Design section states it.
   - A map entry that restates an exported doc comment shrinks to a name. Map prose with no other
     home moves to the README or a `docs/` page.
5. Every comment in code, tests, scripts, and configuration is one of the five kinds. Doc comments
   on unexported identifiers and on test functions are gone; reasoning with no other home moves
   into names, exported docs, or `docs/`. The currency script's section comments become named
   functions.
6. Each `.golangci.yml` enables nolintlint with `require-explanation` and `require-specific`.
7. Each STANDARDS.md gains a comment rule that cites the five kinds for source, tests, scripts, and
   every configuration file (TOML, YAML, `.gitignore`).
8. The check passes locally and in CI in all three repositories. No CHANGELOG changes: no module's
   `go.mod` requirements change.

**Test seams**
- `mise run check` and `mise run currency` in each repository; the existing suites are the
  regression seam.

**Slices**
1. go-database upgrade: mise-action v5.2.10.
2. go-observability upgrade: the same.
3. go-cli-sdk upgrade: the same.
4. go-database: pin, check wiring, and context.
5. go-database: comments, nolintlint, and STANDARDS comment rule.
6. go-observability: pin, check wiring, and context, including the logs-pipeline note.
7. go-observability: comments, nolintlint, and STANDARDS comment rule.
8. go-cli-sdk: pin, check wiring, and context.
9. go-cli-sdk: comments, nolintlint, and STANDARDS comment rule.

**Out of scope**

Building the logs pipeline, rewriting exported doc comments, a Markdown link check, budgeting
`docs/`, releases and tags, feature or API changes, the Go templates.

**Door**

Two-way: every change sits on branches and pull requests, and nothing is tagged.

## Progress

slices 0/9 committed · standards — · spec — · editor —

## Decisions

- adopt: CHANGELOG entries record only changes to a module's `go.mod` requirements. (wave 1, W2)

## Pending edits

None.
