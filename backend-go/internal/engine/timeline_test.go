package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func emptyFacts() TimelineFacts {
	return TimelineFacts{Deletions: map[string]int{}, DeletedWithEvent: map[int64]bool{}, ServicesAtRisk: map[string]int{}}
}

// scoreOf finds the unrounded score of a service.
func scoreOf(t *testing.T, r *Result, serviceID int64) float64 {
	t.Helper()
	for _, cr := range r.Customers {
		for _, sr := range cr.Services {
			if sr.Service.ID == serviceID {
				require.True(t, sr.Scored, "service %d is live", serviceID)
				return sr.ScoreRaw
			}
		}
	}
	t.Fatalf("service %d not found", serviceID)
	return 0
}

func pointsByDay(points []TimelinePoint) map[string]TimelinePoint {
	out := map[string]TimelinePoint{}
	for _, p := range points {
		out[p.Day] = p
	}
	return out
}

func TestTimelineRange(t *testing.T) {
	now := time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC)
	from, to := TimelineRange(now, 90)
	assert.Equal(t, time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC), from, "89 days back plus today")
	assert.Equal(t, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), to)

	from, to = TimelineRange(now, 0)
	assert.Equal(t, from, to, "no fact days: an empty range")
}

// The shape: pastDays fact points ending today, then futureDays forecast points, one per day.
func TestTimelineDays(t *testing.T) {
	r := calc(t, nil)
	require.Equal(t, time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC), r.Now.Truncate(time.Second))

	points := r.Timeline(emptyFacts(), 90, 30)
	require.Len(t, points, 120)
	assert.Equal(t, TimelinePoint{Day: "2026-07-22", Kind: TimelineFact}, points[0])
	assert.Equal(t, "2026-10-19", points[89].Day)
	assert.Equal(t, TimelineFact, points[89].Kind, "today is a fact day")
	assert.Equal(t, "2026-10-20", points[90].Day)
	assert.Equal(t, TimelineForecast, points[90].Kind)
	assert.Equal(t, "2026-11-18", points[119].Day)
	for i := 1; i < len(points); i++ {
		assert.Less(t, points[i-1].Day, points[i].Day, "ascending, no gaps or repeats")
	}

	assert.Len(t, r.Timeline(emptyFacts(), 0, 1), 1, "pastDays=0: forecast only")
}

// The forecast on data.json (positions checked by hand against the data):
// tomorrow gets every service whose date has passed or falls later today — the prolong_wait ones
// (expiresAt + 5 days, long gone), the suspended ones that expired, and the hourly service 1
// expiring today at 15:30. 2026-10-26 has service 26, 2026-11-02 service 4; service 21
// (2026-11-19) is the 31st day, outside futureDays=30.
func TestTimelineForecast(t *testing.T) {
	r := calc(t, nil)
	days := pointsByDay(r.Timeline(emptyFacts(), 0, 30))

	sum := func(ids ...int64) float64 {
		var s float64
		for _, id := range ids {
			s += scoreOf(t, r, id) / 100
		}
		return s
	}
	assert.InDelta(t, sum(1, 5, 9, 10, 14, 15, 19, 20, 25, 30), days["2026-10-20"].ExpectedNonRenewals, 1e-9)
	assert.InDelta(t, sum(26), days["2026-10-26"].ExpectedNonRenewals, 1e-9)
	assert.InDelta(t, sum(4), days["2026-11-02"].ExpectedNonRenewals, 1e-9)

	var total float64
	for _, p := range days {
		total += p.ExpectedNonRenewals
	}
	assert.InDelta(t, sum(1, 5, 9, 10, 14, 15, 19, 20, 25, 30, 26, 4), total, 1e-9, "nothing else in 30 days")
	assert.Greater(t, days["2026-10-20"].ExpectedNonRenewals, 4.0, "prolong_wait services are near certain losses")

	// 31 days reach service 21.
	days = pointsByDay(r.Timeline(emptyFacts(), 0, 31))
	assert.InDelta(t, sum(21), days["2026-11-19"].ExpectedNonRenewals, 1e-9)
}

// A weight change moves the forecast: it is built from the same scores as the rest of the model.
func TestTimelineForecastFollowsWeights(t *testing.T) {
	base := pointsByDay(calc(t, nil).Timeline(emptyFacts(), 0, 30))
	lower := pointsByDay(calc(t, func(s *Settings) {
		s.Features["prolong_wait"] = FeatureSetting{Weight: 0, Enabled: false}
	}).Timeline(emptyFacts(), 0, 30))
	assert.Less(t, lower["2026-10-20"].ExpectedNonRenewals, base["2026-10-20"].ExpectedNonRenewals)
}

func TestTimelineFacts(t *testing.T) {
	r := calc(t, nil)
	facts := emptyFacts()
	facts.Deletions["2026-10-18"] = 2
	facts.Deletions["2026-01-01"] = 5 // outside 90 days: the store would not return it, the engine ignores it
	facts.ServicesAtRisk["2026-10-18"] = 9
	facts.ServicesAtRisk["2026-10-19"] = 0 // a real zero, not "no data"

	days := pointsByDay(r.Timeline(facts, 90, 30))
	assert.Equal(t, 2, days["2026-10-18"].Deletions)
	require.NotNil(t, days["2026-10-18"].ServicesAtRisk)
	assert.Equal(t, 9, *days["2026-10-18"].ServicesAtRisk)
	require.NotNil(t, days["2026-10-19"].ServicesAtRisk)
	assert.Zero(t, *days["2026-10-19"].ServicesAtRisk)
	assert.Nil(t, days["2026-10-17"].ServicesAtRisk, "no risk_daily row → nil")
	assert.Nil(t, days["2026-10-20"].ServicesAtRisk, "forecast has no history")
	_, ok := days["2026-01-01"]
	assert.False(t, ok)
}

// Deleted services of the source without a recorded deletion are dated expiresAt + 5 days.
// On data.json within 90 days that is service 18 (expired 2026-09-14 → 2026-09-19); the others
// are older. A recorded deletion replaces the estimate.
func TestTimelineEstimatedDeletions(t *testing.T) {
	r := calc(t, nil)
	days := pointsByDay(r.Timeline(emptyFacts(), 90, 30))
	total := 0
	for _, p := range days {
		total += p.DeletionsEstimated
	}
	assert.Equal(t, 1, total)
	assert.Equal(t, 1, days["2026-09-19"].DeletionsEstimated)

	days = pointsByDay(r.Timeline(emptyFacts(), 400, 1))
	total = 0
	for _, p := range days {
		total += p.DeletionsEstimated
	}
	assert.Equal(t, 6, total, "all six deleted services of data.json within 400 days")

	facts := emptyFacts()
	facts.DeletedWithEvent[18] = true
	days = pointsByDay(r.Timeline(facts, 90, 30))
	assert.Zero(t, days["2026-09-19"].DeletionsEstimated, "the event is counted in Deletions instead")
}
