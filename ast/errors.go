package ast

import (
	"errors"
	"fmt"
)

// ErrInvalidStatement classifies a statement that cannot be rendered — an
// empty select list, a derived table without an alias, paging without ORDER
// BY. It is wrapped with the specific defect, so errors.Is matches the class
// while the message names the fix.
var ErrInvalidStatement = errors.New("invalid query statement")

// FeatureReturning names the declared native returning clause, carried by
// [UnsupportedFeatureError] when a dialect lacks [ReturningRenderer].
const FeatureReturning = "returning"

// UnsupportedFeatureError reports a well-formed statement using a declared
// native feature the dialect does not implement. Unlike
// [ErrInvalidStatement] the statement itself is sound; the engine has no
// rendering for the feature, and the vocabulary fails typed rather than
// emitting SQL the engine would reject.
type UnsupportedFeatureError struct {
	Feature string
	Dialect string
}

func (e *UnsupportedFeatureError) Error() string {
	return fmt.Sprintf("dialect %q does not support %s", e.Dialect, e.Feature)
}
