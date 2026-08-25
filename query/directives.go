package query

// Page is the 1-based page directive of a read: which page, and how many
// rows per page. Both fields must be at least 1.
type Page struct {
	Number int
	Size   int
}

// Sort is one sort directive, naming a projected field.
type Sort struct {
	Field      string
	Descending bool
}

// Op is a filter directive's operator. The values are short strings so a
// read contract can carry them verbatim.
type Op string

const (
	OpEq        Op = "eq"
	OpNe        Op = "ne"
	OpGt        Op = "gt"
	OpGe        Op = "ge"
	OpLt        Op = "lt"
	OpLe        Op = "le"
	OpLike      Op = "like"
	OpIsNull    Op = "null"
	OpIsNotNull Op = "notnull"
	OpIn        Op = "in"
)

// Filter is one filter directive: a projected field name, an operator, and
// the value. [OpIsNull] and [OpIsNotNull] ignore Value; [OpIn] requires
// Value to be a []any.
type Filter struct {
	Field string
	Op    Op
	Value any
}

// Directives is one read request against a projection. Filters and Sort
// reference projected field names — an unknown name is an
// [UnknownFieldError], never SQL. Where optionally carries a composed
// [Predicate] beyond the field vocabulary, ANDed with the filters: the door
// back down to the composition core.
type Directives struct {
	Page    Page
	Sort    []Sort
	Filters []Filter
	Where   Predicate
}
