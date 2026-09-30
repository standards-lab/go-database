package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/standards-lab/sqlate"

	"github.com/standards-lab/go-database"
)

// A connectivity failure from Start or Ping carries the sentinel, which is
// sqlate's, and the driver's own error, each matchable through errors.Is.
func TestErrConnectionFailed_DualWrapFromStartAndPing(t *testing.T) {
	if database.ErrConnectionFailed != sqlate.ErrConnectionFailed {
		t.Fatal("ErrConnectionFailed is not sqlate's sentinel")
	}

	connector := &stubConnector{}
	db := newTestDB(t, connector)
	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	connector.fail.Store(true)
	pingErr := db.Ping(context.Background())

	failed := &stubConnector{}
	failed.fail.Store(true)
	startErr := newTestDB(t, failed).Start(context.Background())

	for name, err := range map[string]error{"Start": startErr, "Ping": pingErr} {
		if !errors.Is(err, database.ErrConnectionFailed) || !errors.Is(err, sqlate.ErrConnectionFailed) {
			t.Errorf("%s = %v, want ErrConnectionFailed", name, err)
		}
		if !errors.Is(err, errDial) && !errors.Is(err, errPing) {
			t.Errorf("%s = %v, want the driver's error in the chain", name, err)
		}
		if errors.Is(err, database.ErrNotReady) {
			t.Errorf("%s = %v, want the sentinels distinct", name, err)
		}
	}
}
