package database

import (
	"errors"

	"github.com/standards-lab/sqlate"
)

var (
	// ErrNotReady reports a call against a [DB] before a successful Start
	// or after Shutdown.
	ErrNotReady = errors.New("database not ready")

	// ErrConnectionFailed reports a failure to reach the database; it is
	// sqlate's sentinel.
	ErrConnectionFailed = sqlate.ErrConnectionFailed
)
