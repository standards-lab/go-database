package pgvector

import (
	"errors"
	"fmt"

	sqlvector "github.com/standards-lab/sqlate/postgres/pgvector"
)

// Metric is sqlate's pgvector metric: its name, its distance operator, and
// its index operator class.
type Metric = sqlvector.Metric

// ErrUnknownMetric reports a metric name that is none of sqlate's metrics.
var ErrUnknownMetric = errors.New("unknown pgvector metric")

// ParseMetric returns the metric of sqlate's table whose Name is name,
// matched exactly, without I/O. Any other name returns an error that wraps
// [ErrUnknownMetric] and quotes name.
func ParseMetric(name string) (Metric, error) {
	for _, m := range sqlvector.Metrics() {
		if m.Name == name {
			return m, nil
		}
	}
	return Metric{}, fmt.Errorf("%w: %q", ErrUnknownMetric, name)
}
