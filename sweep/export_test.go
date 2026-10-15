package sweep

import (
	"time"

	"github.com/standards-lab/go-core/reactor"
)

// Handle returns the Func the sweeper's reactor runs, so a test delivers
// occurrences at the times it chooses, without the source's clock.
func Handle(s *Sweeper) reactor.Func[time.Time] { return s.handle }

// Tick returns the interval of the sweeper's source.
func Tick(s *Sweeper) time.Duration { return s.tick }
