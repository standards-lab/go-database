package postgres_test

import (
	"os"
	"testing"
	"time"
	_ "time/tzdata"
)

const nonUTCLocalZone = "Europe/London"

func TestMain(m *testing.M) {
	local, err := time.LoadLocation(nonUTCLocalZone)
	if err != nil {
		panic(err)
	}
	time.Local = local
	os.Exit(m.Run())
}
