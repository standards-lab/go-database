# The write path

Candidate direction for writes, migrated from the predecessor repository's planning. It is unbuilt;
a build session settles the API in plan mode before anything lands, and the roadmap re-plan decides
whether it is next.

## Evaluate the engines first

As for the query vocabulary (`query.md`), the build session evaluates idiomatic data-mutation
patterns across the common engines — Postgres, SQL Server, MySQL, SQLite, Oracle — and sorts them
into the patterns every engine implements the same way, which become the standard tier of the write
path, and the native patterns worth reaching for in each engine, with the methods `Dialect` would
need to render them (`../design/dialect.md`): how an engine returns the rows a write affected
(`RETURNING` against `OUTPUT`), how it upserts, how it reports a constraint violation.

## Transactions

All mutations against one database instance in a unit of work run inside one transaction, so the
unit either commits completely or fails completely. The base's write helpers take the transaction
rather than opening one; the consumer owns the transaction's boundary, as `seed` already does with
one transaction per step.

## Optimistic concurrency

The base fixes a result contract for an update or delete guarded by a version: the statement yields
one row `(version, found, matched)`, and a scan helper maps the outcomes to `sql.ErrNoRows`, a
version-mismatch sentinel, or the new version. The SQL text stays with the consumer, written for its
engine.

## The error model

Sentinels for the constraint-violation classes — unique, foreign-key, check, not-null — join
`ErrNotReady` and `ErrConnectionFailed` in the base, and a violation type carries the constraint name
while wrapping both the sentinel and the driver error, so `errors.Is` matches the class, `errors.As`
reaches the driver's own error, and the constraint name survives for mapping. Each provider's
`MapError` wraps the base sentinels; an unclassified driver error still reaches the caller through
the wrap. `sql.ErrNoRows` flows through reads unchanged.

## Open questions

- The outcome of the engine evaluation: which mutation patterns are standard, which are native, and
  what `Dialect` grows to hold.
- Where the typed write-path errors (conflict, concurrency, validation) live, and how their field
  maps project into a consumer's error responses; a structural interface the web tier consumes is
  the candidate.
