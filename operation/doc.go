// Package operation is the base module's contract layer: constructors
// shaped by what a consumer means to do, lowering to rendered statements
// from the ast package's vocabulary. The query side is a [Projection] of
// name-to-expression fields with its two queries — [Projection.List]
// lowers [Directives] of page, sort, and operator-keyed filters into a
// count-and-page pair over one WHERE clause, and [Projection.One] forms the
// single-row read. The command side is [Insertion], [GuardedUpdate], and
// [GuardedDelete]: the identity-returning insert and the two
// optimistic-concurrency commands, each lowering through its SQL method.
// The exec package runs what this layer produces.
//
// # The field contract
//
// Directives reference a projection's field names, never SQL. An unknown
// name or operator is a typed error — [UnknownFieldError],
// [UnknownOperatorError] — the consumer maps to its own response, and the
// projection's key field joins every ORDER BY as the tie-breaker, so offset
// paging is stable.
//
// # The guard contract
//
// A guarded command matches its row by key and expected version and
// increments the version itself, so the new version is deterministic. The
// [Guard] names the consumer's version column — the library fixes the
// mechanism, never the schema — and the [Guarded] pair keeps the contract
// in pure standard SQL: the command first, the check only when no row was
// touched, splitting not-found from version-mismatch. [Insertion] is the
// one command needing a dialect capability: its returning clause renders
// only through ast.ReturningRenderer and fails typed without it.
package operation
