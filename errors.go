package database

import "errors"

var (
	// ErrNotReady reports a call against a [DB] before a successful Start
	// or after Shutdown.
	ErrNotReady = errors.New("database not ready")

	// ErrConnectionFailed classifies a connectivity failure. It is wrapped
	// alongside the driver's error in the dual form
	// fmt.Errorf("%w: %w", ErrConnectionFailed, err), so errors.Is matches
	// the class while the cause stays recoverable.
	ErrConnectionFailed = errors.New("database connection failed")
)
