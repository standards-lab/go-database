package database_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/standards-lab/go-database"
)

func TestSentinels(t *testing.T) {
	if got := database.ErrNotReady.Error(); got != "database not ready" {
		t.Errorf("ErrNotReady = %q", got)
	}
	if got := database.ErrConnectionFailed.Error(); got != "database connection failed" {
		t.Errorf("ErrConnectionFailed = %q", got)
	}
}

// The dual-wrap form is the package's error contract: errors.Is classifies by
// sentinel while the driver's cause stays wrapped and matchable.
func TestSentinels_DualWrap(t *testing.T) {
	cause := errors.New("connection refused")
	err := fmt.Errorf("%w: %w", database.ErrConnectionFailed, cause)

	if !errors.Is(err, database.ErrConnectionFailed) {
		t.Error("errors.Is(err, ErrConnectionFailed) = false")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want the cause to stay matchable")
	}
	if errors.Is(err, database.ErrNotReady) {
		t.Error("errors.Is(err, ErrNotReady) = true, want the sentinels distinct")
	}
}
