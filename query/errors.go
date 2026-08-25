package query

import (
	"errors"
	"fmt"
)

// ErrInvalidStatement classifies a statement that cannot be rendered — an
// empty select list, a derived table without an alias, paging without ORDER
// BY. It is wrapped with the specific defect, so errors.Is matches the class
// while the message names the fix.
var ErrInvalidStatement = errors.New("invalid query statement")

// FieldUse names the directive position where a projected field was
// referenced, carried by [UnknownFieldError].
type FieldUse string

const (
	FieldUseSort   FieldUse = "sort"
	FieldUseFilter FieldUse = "filter"
)

// UnknownFieldError reports a sort or filter directive naming a field the
// projection does not declare. It is the field contract's boundary: the name
// never reaches the SQL, and a consumer maps the error to its own response
// with errors.As.
type UnknownFieldError struct {
	Field string
	Use   FieldUse
}

func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("unknown %s field %q", e.Use, e.Field)
}

// UnknownOperatorError reports a filter directive carrying an operator the
// vocabulary does not define, mapped by the consumer the same way as
// [UnknownFieldError].
type UnknownOperatorError struct {
	Op Op
}

func (e *UnknownOperatorError) Error() string {
	return fmt.Sprintf("unknown filter operator %q", string(e.Op))
}
