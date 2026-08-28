// Package ast is the base module's statement layer: standard SQL composed
// as Go values — an abstract syntax tree rendered through the provider's
// dialect. The vocabulary is sealed: expressions ([Col], [Val], [Fn], and
// [Raw] as the deliberate escape hatch), predicates (the comparison methods
// on [Expression], combined by [And], [Or], [Not]), and statements as plain
// structs — [Select] and [Compound] as the [Query] expressions, [Insert],
// [Update], and [Delete] as the write statements — each rendered by its
// Render method into an [SQL] value: the text and bound arguments,
// placeholders through the dialect. The operation package composes
// contract-shaped constructors above this layer; the exec package runs what
// either produces.
//
// # Standard core
//
// The vocabulary composes the SQL every engine accepts, and the portability
// rules are structural rather than documented: table aliases render without
// AS, compound branches render without parentheses and carry no tail
// clauses, WITH belongs to the outermost statement, a subquery orders only
// when it pages, write statements render only at the top and take plain
// table names, and paging is the SQL:2008 OFFSET/FETCH form with bound
// arguments, always under an ORDER BY. Request-derived text reaches a
// statement only as a bound argument; [Raw] and the pool beneath the
// package are the declared passthroughs for SQL the vocabulary does not
// compose.
//
// # Dialect capabilities
//
// Where engines diverge, the package defines an optional dialect interface
// checked at the divergent render site, in one of two directions. A
// standard feature with divergent renderings keeps standard emission as the
// default and lets a dialect override it — [PagingRenderer]. A declared
// native feature with no standard emission renders only through its
// capability and otherwise fails with a typed [UnsupportedFeatureError] —
// [ReturningRenderer], for the write statements' Returning clause. Either
// way the dialect interface itself grows nothing speculatively, and a
// statement renders portably or fails typed; it never silently emits
// one-engine SQL.
package ast
