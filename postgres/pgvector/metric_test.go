package pgvector_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/standards-lab/go-database/postgres/pgvector"
	sqlvector "github.com/standards-lab/sqlate/postgres/pgvector"
)

func TestParseMetric_ResolvesEachMetricByItsName(t *testing.T) {
	for _, want := range sqlvector.Metrics() {
		got, err := pgvector.ParseMetric(want.Name)
		if err != nil {
			t.Errorf("ParseMetric(%q): %v", want.Name, err)
			continue
		}
		if got != want {
			t.Errorf("ParseMetric(%q) = %+v, want %+v", want.Name, got, want)
		}
	}
}

func TestParseMetric_RefusesANameOutsideTheTable(t *testing.T) {
	for _, name := range []string{"manhattan", "Cosine", ""} {
		got, err := pgvector.ParseMetric(name)
		if !errors.Is(err, pgvector.ErrUnknownMetric) {
			t.Errorf("ParseMetric(%q) error = %v, want ErrUnknownMetric", name, err)
			continue
		}
		if !strings.Contains(err.Error(), `"`+name+`"`) {
			t.Errorf("ParseMetric(%q) error = %q, want it to name the input", name, err)
		}
		if got != (pgvector.Metric{}) {
			t.Errorf("ParseMetric(%q) = %+v, want the zero Metric", name, got)
		}
	}
}
