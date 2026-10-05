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

	cases := []struct {
		name  string
		err   error
		cause error
	}{
		{"Start", startErr, errDial},
		{"Ping", pingErr, errPing},
	}
	for _, tc := range cases {
		if !errors.Is(tc.err, database.ErrConnectionFailed) || !errors.Is(tc.err, sqlate.ErrConnectionFailed) {
			t.Errorf("%s = %v, want ErrConnectionFailed", tc.name, tc.err)
		}
		if !errors.Is(tc.err, tc.cause) {
			t.Errorf("%s = %v, want the driver's %v in the chain", tc.name, tc.err, tc.cause)
		}
		if errors.Is(tc.err, database.ErrNotReady) {
			t.Errorf("%s = %v, want the sentinels distinct", tc.name, tc.err)
		}
	}
}
