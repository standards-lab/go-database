# The query vocabulary

Candidate direction for reads, migrated from the predecessor repository's planning. It is unbuilt;
a build session settles the API in plan mode before anything lands, and the roadmap re-plan decides
whether it is next.

## Evaluate the engines first

Before the API is designed, the build session evaluates idiomatic query patterns across the common
engines — Postgres, SQL Server, MySQL, SQLite, Oracle — and sorts them into two sets: the patterns
every engine implements the same way, which become the standard tier of the query vocabulary, and
the native patterns worth reaching for in each engine, with the methods `Dialect` would need so the
builder can render them (`../design/dialect.md`). The aim is the most capable vocabulary that is
common to every engine, plus deliberate native reach, rather than the intersection of what the
engines share.

## Direction

A `query` package in the base module holds the persistence query vocabulary: page and sort
directives, exact-match filter directives keyed by projected field names, and a page result. A
projection declares fields as name-to-expression pairs with one key field, and a builder generates
standard SQL from a projection and a set of directives, rendering only bind placeholders through the
dialect.

Decisions already taken:

- Paging is emitted in the SQL:2008 form `OFFSET n ROWS FETCH NEXT m ROWS ONLY`, so `Dialect` gains
  no paging method until a provider is built for an engine that lacks the form.
- The key field is appended to every ORDER BY as the tie-breaker, so offset paging is stable.
- A field's expression may come from a derived table in the projection's FROM; that is how a
  consumer projects a computed column without the builder knowing.
- Count and page are two statements the consumer runs.
- An unknown sort or filter field is a typed error the consumer maps to its own response.
- The HTTP side of paging (parsing `page`, `size`, and `sort` from a request; writing the page
  envelope) belongs to the web tier, not here. This package owns the directives; the consumer
  translates.

## Open questions

- The outcome of the engine evaluation: which patterns are standard, which are native, and what
  `Dialect` grows to hold.
- The builder API: projection construction, filter directives, sort validation, the exact error
  type for an unknown field.
