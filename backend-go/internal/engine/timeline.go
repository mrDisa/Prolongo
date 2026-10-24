package engine

import (
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

// Timeline point kinds (docs/ОБЩЕЕ_ТЗ.md §7.7).
const (
	TimelineFact     = "fact"
	TimelineForecast = "forecast"
)

// TimelineFacts is what the database knows about the past (store.TimelineFacts).
// Days are "YYYY-MM-DD" in UTC.
type TimelineFacts struct {
	Deletions        map[string]int // day → transitions to deleted recorded in status_events
	DeletedWithEvent map[int64]bool // services whose deletion is recorded, on any day
	ServicesAtRisk   map[string]int // day → core.risk_daily.services_at_risk
}

// TimelinePoint is one day of the churn chart.
type TimelinePoint struct {
	Day  string // YYYY-MM-DD, UTC
	Kind string // fact | forecast

	// fact
	Deletions          int  // recorded transitions to deleted
	DeletionsEstimated int  // deleted services without a recorded event, dated DeletionDue
	ServicesAtRisk     *int // nil when core.risk_daily has no row for the day

	// forecast
	ExpectedNonRenewals float64 // Σ p of live services whose term ends that day
}

// TimelineRange is the time span of the fact part: from the start of the first fact day
// to the start of tomorrow. The store reads facts in [from, to).
func TimelineRange(now time.Time, pastDays int) (from, to time.Time) {
	tomorrow := startOfDay(now).AddDate(0, 0, 1)
	return tomorrow.AddDate(0, 0, -pastDays), tomorrow
}

// Timeline builds one point per day: pastDays of fact ending with the day of r.Now
// (today included), then futureDays of forecast starting tomorrow.
//
// Forecast: a live service's term ends on ExpiresAt (active, suspended) or on DeletionDue
// (prolong_wait); a date already passed moves to tomorrow, since the service has not left yet.
// Its probability ScoreRaw / 100 goes to that day.
func (r *Result) Timeline(facts TimelineFacts, pastDays, futureDays int) []TimelinePoint {
	today := startOfDay(r.Now)
	tomorrow := today.AddDate(0, 0, 1)

	points := make([]TimelinePoint, 0, pastDays+futureDays)
	index := make(map[string]int, pastDays+futureDays) // day → position in points
	add := func(p TimelinePoint) {
		index[p.Day] = len(points)
		points = append(points, p)
	}
	for i := pastDays - 1; i >= 0; i-- {
		day := dayKey(today.AddDate(0, 0, -i))
		p := TimelinePoint{Day: day, Kind: TimelineFact, Deletions: facts.Deletions[day]}
		if n, ok := facts.ServicesAtRisk[day]; ok {
			p.ServicesAtRisk = &n // n is a new variable on every iteration (Go 1.22+)
		}
		add(p)
	}
	for i := range futureDays {
		add(TimelinePoint{Day: dayKey(tomorrow.AddDate(0, 0, i)), Kind: TimelineForecast})
	}

	for ci := range r.Customers {
		for _, sr := range r.Customers[ci].Services {
			s := sr.Service
			switch {
			case s.Status == domain.StatusDeleted && !facts.DeletedWithEvent[s.ID]:
				if i, ok := index[dayKey(s.DeletionDue())]; ok && points[i].Kind == TimelineFact {
					points[i].DeletionsEstimated++
				}
			case sr.Scored:
				due := s.ExpiresAt
				if s.Status == domain.StatusProlongWait {
					due = s.DeletionDue()
				}
				if due.Before(tomorrow) {
					due = tomorrow
				}
				if i, ok := index[dayKey(due)]; ok && points[i].Kind == TimelineForecast {
					points[i].ExpectedNonRenewals += sr.ScoreRaw / 100
				}
			}
		}
	}
	return points
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func dayKey(t time.Time) string {
	return t.UTC().Format(time.DateOnly)
}
