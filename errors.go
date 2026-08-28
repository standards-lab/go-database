package database

import (
	"errors"
	"fmt"
)

var (
	// ErrNotReady reports a call against a [DB] before a successful Start
	// or after Shutdown.
	ErrNotReady = errors.New("database not ready")

	// ErrConnectionFailed classifies a connectivity failure. It is wrapped
	// alongside the driver's error in the dual form
	// fmt.Errorf("%w: %w", ErrConnectionFailed, err), so errors.Is matches
	// the class while the cause stays recoverable.
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrUniqueViolation classifies a unique-constraint violation. A
	// provider's MapError wraps it in a [ConstraintError]; errors.Is
	// matches the class.
	ErrUniqueViolation = errors.New("unique constraint violation")

	// ErrForeignKeyViolation classifies a foreign-key-constraint violation,
	// wrapped the same way as [ErrUniqueViolation].
	ErrForeignKeyViolation = errors.New("foreign key constraint violation")

	// ErrCheckViolation classifies a check-constraint violation, wrapped
	// the same way as [ErrUniqueViolation].
	ErrCheckViolation = errors.New("check constraint violation")

	// ErrNotNullViolation classifies a not-null-constraint violation,
	// wrapped the same way as [ErrUniqueViolation].
	ErrNotNullViolation = errors.New("not-null constraint violation")

	// ErrVersionMismatch reports an optimistic-concurrency guard that
	// matched its key but not its expected version. The execution layer's
	// guarded runners return it wrapped with the expected and current
	// versions; errors.Is matches the class.
	ErrVersionMismatch = errors.New("version mismatch")
)

// ConstraintError is a classified constraint violation: the class sentinel,
// the driver's own error, and the violated constraint's name when the driver
// exposes it. Unwrap yields both wrapped errors, so errors.Is matches the
// class while errors.As still reaches the driver error, and the constraint
// name survives for a consumer's field mapping.
type ConstraintError struct {
	Constraint string
	Class      error
	Err        error
}

func (e *ConstraintError) Error() string {
	if e.Constraint == "" {
		return fmt.Sprintf("%v: %v", e.Class, e.Err)
	}
	return fmt.Sprintf("%v on constraint %q: %v", e.Class, e.Constraint, e.Err)
}

func (e *ConstraintError) Unwrap() []error {
	return []error{e.Class, e.Err}
}
