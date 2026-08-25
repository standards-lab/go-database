// Package query is the base module's persistence query vocabulary: standard
// SQL composed as Go values, rendered through the provider's dialect. It has
// two levels. The composition core is a sealed vocabulary of expressions
// ([Col], [Val], [Fn], and [Raw] as the deliberate escape hatch), predicates
// (the comparison methods on [Expression], combined by [And], [Or], [Not]),
// and statements as plain structs — [Select] and [Compound], with [CTE]
// lists — rendered by [Select.SQL] into SQL text and bound arguments,
// placeholders through the dialect. The read vocabulary sits on top: a
// [Projection] of name-to-expression fields and [Directives] of page, sort,
// and operator-keyed filters, lowered by [Projection.Statements] into a
// count-and-page statement pair over one WHERE clause.
//
// # Standard SQL only
//
// The vocabulary composes the SQL every engine accepts, and the portability
// rules are structural rather than documented: table aliases render without
// AS, compound branches render without parentheses and carry no tail
// clauses, WITH belongs to the outermost statement, and paging is the
// SQL:2008 OFFSET/FETCH form with bound arguments, always under an ORDER BY.
// Where an engine lacks a standard form, the package defines an optional
// dialect extension checked at the divergent render site — [PagingRenderer]
// is the worked case — and standard emission stays the default. The dialect
// itself grows nothing speculatively.
//
// # The field contract
//
// Directives reference a projection's field names, never SQL. An unknown
// name or operator is a typed error — [UnknownFieldError],
// [UnknownOperatorError] — the consumer maps to its own response, and the
// projection's key field joins every ORDER BY as the tie-breaker, so offset
// paging is stable. Request-derived text reaches a statement only as a bound
// argument.
package query
