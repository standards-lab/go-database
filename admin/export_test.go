package admin

import (
	"context"
	"time"
)

// SetProbe replaces Ready's schema verification, so a test can hold a probe
// in flight or count the probes that run.
func SetProbe(s *Service, probe func(context.Context) error) { s.probe = probe }

// ExpireThrottle lets the next probe run as though the interval had passed.
func ExpireThrottle(s *Service) { s.probed.Store(int64(time.Since(s.base) - reverifyInterval)) }
