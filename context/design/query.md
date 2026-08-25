# The query vocabulary

Settled in the `query-vocabulary` session (2026-08-25, `v1.data.reads.query`). The built package
is authoritative for everything it expresses; this note holds the settled intent the code cannot
show — the boundaries, the growth path, and the extension pattern the next slices build on. The
landing zone owes the package a design page; that debt is flagged at this session's close.

## The permanent split of labor

The package is the runtime half of the data layer's composition story, and that position is
permanent, not transitional. Statements as values are the layer request-shaped work composes at
runtime — filters, sorts, paging known only when the request arrives. The SQL meta language
concept (`docs/context/concepts/sql-meta-language.md`) is the possible authoring surface above
it, compiling ahead-of-time definitions down to these same statement values; it would displace
static-query authoring, never this API. Nothing in v1 waits on it.

## The DML statement family

The core grows one statement shape at a time, on the same expressions, predicates, and
renderer:

- `Select` and `Compound` are built. `Insert`, `Update`, and `Delete` are the writes task's
  (`v1.data.tasks.writes`) — sibling struct values implementing the same sealed `Query`
  contract, their WHERE clauses the same predicate trees.
- How write results come back (`RETURNING`, `OUTPUT`) is engine-divergent and stays the writes
  task's dialect question.
- No DDL and no metadata tier, permanently: migrations own the schema, and introspection is
  provider-native (INFORMATION_SCHEMA is false-common — Oracle and SQLite diverge).

## The portability promise

A statement the vocabulary accepts renders to SQL that runs on every provider, by
construction. The placement validations that enforce this — WITH only on the outermost
statement, subquery ORDER BY only when paging, compound branches without tail clauses — are
universal and are never relaxed per dialect: relaxation would mint one-engine statements
without a declaration, outside the service-tiers discipline. Engine-specific SQL has its
declared routes: `Raw` inside the vocabulary, the pool beneath it.

## The dialect render extension pattern

Where an engine lacks a standard form, the package defines an optional interface, the
divergent render site type-asserts the dialect, and standard emission is the default. A
provider opts in per divergence; `database.Dialect` itself never grows speculatively, and no
provider is forced to implement rendering. `PagingRenderer` is the built worked case. The
ledger of known divergences, each earning an interface only when a provider is built for an
engine that needs it:

- LIMIT-style paging — MySQL and SQLite lack OFFSET/FETCH (covered by `PagingRenderer`).
- The `RECURSIVE` keyword — SQL Server and Oracle reject it; recursion is implicit.
- `EXCEPT` — older Oracle spells it `MINUS`.
- Identifier quoting — `Col` preserves identifier parts so a quoting extension
  (`IdentifierRenderer`) can render them without parsing; until one exists, parts render
  verbatim dot-joined.

## Held: read-vocabulary pressure points

Carried from the R&D survey, watched rather than built:

- Search (OR-composed pattern matching across fields) stayed out of the directive vocabulary;
  `Directives.Where` composes it from the core when a consumer needs it. If that recurs across
  consumers, it is a candidate directive.
- The directive operator set is the full comparison suite; the survey's evidence was that
  narrowing it (the Go-port regression to exact match) is what strands consumer features.
