package engine

import (
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

// Now picks the moment the model calculates from (docs/ТЗ_BACKEND_GO.md §9.1):
//   - data  → source time (public.source_clock), else the latest timestamp in the data, else real time;
//   - real  → real time;
//   - fixed → the fixed moment (real time if it is missing).
//
// The organizers' data is frozen in October 2026, so "data" is the default:
// with real time every service would look long expired.
func Now(c Clock, ds *domain.Dataset, real time.Time) time.Time {
	switch c.Mode {
	case ClockReal:
		return real.UTC()
	case ClockFixed:
		if c.FixedAt != nil {
			return c.FixedAt.UTC()
		}
		return real.UTC()
	default:
		if ds != nil && ds.SourceTime != nil {
			return ds.SourceTime.UTC()
		}
		if ds != nil && !ds.LatestTimestamp.IsZero() {
			return ds.LatestTimestamp
		}
		return real.UTC()
	}
}
