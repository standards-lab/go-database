# The layer ontology

Settled in the `writes-vocabulary` session (2026-08-28, `v1.data.writes.database`), absorbing
and superseding the query-vocabulary note. The built packages are authoritative for everything
they express; this note holds the settled intent the code cannot show — the boundaries, the
rationale, and the seams the next slices build on.

> Superseded (2026-08-31): the DSL strategy (`standards-lab
> context/design/dsl-driven-services.md`, executed by `v1.data.sql`) retires the statement
> vocabulary — `ast`, `operation`, and `exec` are replaced in v0.4 by authored SQL files and
> the thin `query`/`migrate` mechanisms. The permanence claim below is withdrawn; the parts
> that survive (the dialect capability pattern and divergence ledger, the portability promise
> in its by-discipline form, baseline-standard ownership, validation-first, the guard
> contract, the typed directive errors) are restated when the executing session rewrites this
> note. The text below describes v0.3.0 as built.

## Four layers, one direction

`database` (service) → `ast` (statements as values) → `operation` (contracts) → `exec`
(execution). Each package names its layer in its `doc.go`; dependencies point one way. The
reading rules: in `ast` a statement is found by its SQL keyword (`select.go`, `insert.go`);
in `operation` a contract by its CQRS side (`query.go`, `command.go`), with the shared field
vocabulary in `fields.go`; `nodes.go` and `renderer.go` are `ast`'s internal machine, and
`dialect.go` in each package is its dialect-facing surface. `exec` is the only layer that
touches database/sql at runtime, and the runner — not the statement pair behind it — is the
contract: a provider-native fast path can replace the portable lowering without a consumer
noticing.

## The permanent split of labor

The `ast` layer's position is permanent, not transitional: statements as values are what
request-shaped work composes at runtime. The SQL meta language concept
(`docs/context/concepts/sql-meta-language.md`) is the possible authoring surface above it,
compiling ahead-of-time definitions down to these same statement values; it would displace
static-query authoring, never this API. Nothing in v1 waits on it.

The sealed `Query` interface deliberately models SQL grammar's query expression — `Select`
and `Compound` only, the marker-method sum-type idiom of go/ast — so every position grammar
restricts to a query (CTE, derived table, subquery operand, `INSERT ... SELECT`) is enforced
at compile time. The write statements are renderable but never `Query`.

## The portability promise, refined

A statement the vocabulary accepts renders to SQL that runs on every provider, or fails
typed — it never silently emits one-engine SQL. The dialect capability pattern has two
directions, distinguished by their default:

- **Override** — a standard feature with divergent renderings: standard emission is the
  default, a dialect opts in to replace it. `PagingRenderer` is the worked case.
- **Declared native** — a feature with no standard emission: rendering goes only through the
  capability, and its absence is a typed `UnsupportedFeatureError`. `ReturningRenderer` is
  the worked case; the engine evaluation found no common form for returning written rows
  (Postgres/SQLite tail `RETURNING`, SQL Server mid-statement `OUTPUT`, MySQL none, Oracle
  out-binds).

The placement validations stay universal and are never relaxed per dialect. Engine-specific
SQL keeps its declared routes: `Raw` inside the vocabulary, the pool beneath it.

### Divergence ledger

Known divergences, each earning an interface (or a hook position) only when a provider is
built for an engine that needs it:

- LIMIT-style paging — MySQL and SQLite lack OFFSET/FETCH (covered by `PagingRenderer`).
- Returning written rows — covered by `ReturningRenderer` at the tail hook; SQL Server's
  `OUTPUT` is positional (mid-statement) and Oracle's `RETURNING ... INTO` is an out-bind,
  not a result set — each waits for its provider.
- Upsert — `ON CONFLICT` vs `ON DUPLICATE KEY` vs `MERGE`; no shape built, deliberately out
  of the writes slice.
- Identity retrieval — `Result.LastInsertId` is MySQL/SQLite-only; the returning form is the
  library's route.
- The `RECURSIVE` keyword — SQL Server and Oracle reject it; recursion is implicit.
- `EXCEPT` — older Oracle spells it `MINUS`.
- Identifier quoting — `Col` preserves identifier parts so a quoting extension
  (`IdentifierRenderer`) can render them without parsing; until one exists, parts render
  verbatim dot-joined.

## The guard contract

The optimistic-concurrency guard is the integer-monotonic mechanism in pure standard SQL:
match by key and expected version, increment by one in the same statement, so the new version
is deterministic and the happy path is one round trip. The check statement runs only on a
miss and splits not-found from version-mismatch; under concurrent writers that classification
is best-effort (the check observes a later snapshot — exact under serializable isolation),
and both outcomes are client-visible conflicts either way. A non-integer concurrency token
(timestamp, rowversion, etag) cannot be incremented by the statement and would arrive as a
sibling declared-native command, not by loosening `Guard.Version`.

`Guard` names the consumer's version column because of the ownership rule below; the service
layer's `Tx.Commit` maps commit errors because Postgres surfaces deferred constraint
violations at COMMIT, and that is the one place they can be classified.

## Principles this session settled

- **Baseline-standard ownership** — an infrastructure-enabling library aligning with an
  external standard takes that standard as its baseline and enables features for the
  architecture without embedding it; organization conventions (the `version` column's name,
  UUID identities) bind at the call sites the organization owns. It is why `Guard.Column`
  exists and why `Insertion`'s identity and version are caller-named fields.
- **Validation-first** — every scope validates the invariants decidable at its entry before
  its first side effect or recursion, so the outermost defect wins under first-failure and
  no computation precedes a decidable rejection. Applied throughout the render layer and the
  operation lowerings; the org-wide enforcement pass and principle page are backlogged at
  the coordinator (`backlog.validation-first`).

## Held seams

- **Transformer decomposition** — `RecursivePath` owns its whole projection because the CTE
  replaces the FROM. A consumer composing several computed fields decomposes it into
  projection transformers, each contributing a CTE and a field; recorded, not built, until a
  second computed-field pattern exists.
- **Search directive** — OR-composed pattern matching across fields stayed out of the
  directive vocabulary; `Directives.Where` composes it from the core. If it recurs across
  consumers, it is a candidate directive.
- The directive operator set remains the full comparison suite; narrowing it is what strands
  consumer features.
