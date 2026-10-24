package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
)

// fakeTimeline: two deletions and services at risk on 2026-10-18; remembers the asked range.
type fakeTimeline struct {
	from, to time.Time
	err      error
}

func (f *fakeTimeline) TimelineFacts(_ context.Context, from, to time.Time) (engine.TimelineFacts, error) {
	f.from, f.to = from, to
	return engine.TimelineFacts{
		Deletions:        map[string]int{"2026-10-18": 2},
		DeletedWithEvent: map[int64]bool{},
		ServicesAtRisk:   map[string]int{"2026-10-18": 9},
	}, f.err
}

func TestChurnTimeline(t *testing.T) {
	d := testDeps(t, nil)
	src := d.Timeline.(*fakeTimeline)
	h := NewRouter(d)
	cookie := login(t, h)

	code, tl := getJSON[timelineJSON](t, h, cookie, "/api/v1/analytics/churn-timeline")
	require.Equal(t, http.StatusOK, code)
	_, s := getJSON[summaryJSON](t, h, cookie, "/api/v1/analytics/summary")
	assert.Equal(t, s.ReferenceTime, tl.ReferenceTime, "the border of fact and forecast is the model's now")
	require.Len(t, tl.Points, 120, "defaults: 90 days of fact, 30 of forecast")
	assert.Equal(t, time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC), src.from)
	assert.Equal(t, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), src.to)

	byDay := map[string]timelinePointJSON{}
	for _, p := range tl.Points {
		byDay[p.Date] = p
	}
	fact := byDay["2026-10-18"]
	assert.Equal(t, "fact", fact.Kind)
	assert.Equal(t, 2, *fact.Deletions)
	assert.Equal(t, 9, *fact.ServicesAtRisk)
	assert.Equal(t, 1, *byDay["2026-09-19"].DeletionsEstimated, "service 18 of data.json")

	tomorrow := byDay["2026-10-20"]
	assert.Equal(t, "forecast", tomorrow.Kind)
	assert.Greater(t, *tomorrow.ExpectedNonRenewals, 4.0)
	assert.Equal(t, *tomorrow.ExpectedNonRenewals, float64(int(*tomorrow.ExpectedNonRenewals*100+0.5))/100, "2 decimals")
}

// The JSON shape of docs/ОБЩЕЕ_ТЗ.md §7.7: fact and forecast carry different fields,
// servicesAtRisk is null when unknown and in every forecast point.
func TestChurnTimelineJSONShape(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodGet, "/api/v1/analytics/churn-timeline?pastDays=3&futureDays=2", "", login(t, h))
	require.Equal(t, http.StatusOK, rec.Code)

	var raw struct {
		Points []map[string]any `json:"points"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Len(t, raw.Points, 5)
	assert.Equal(t, map[string]any{"date": "2026-10-17", "kind": "fact", "deletions": 0.0, "deletionsEstimated": 0.0, "servicesAtRisk": nil}, raw.Points[0])
	assert.Equal(t, map[string]any{"date": "2026-10-18", "kind": "fact", "deletions": 2.0, "deletionsEstimated": 0.0, "servicesAtRisk": 9.0}, raw.Points[1])
	assert.Equal(t, "2026-10-19", raw.Points[2]["date"])
	for _, p := range raw.Points[3:] {
		assert.Equal(t, "forecast", p["kind"])
		assert.Contains(t, p, "expectedNonRenewals")
		assert.Contains(t, p, "servicesAtRisk")
		assert.Nil(t, p["servicesAtRisk"])
		assert.NotContains(t, p, "deletions")
	}
}

func TestChurnTimelineParams(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	_, tl := getJSON[timelineJSON](t, h, cookie, "/api/v1/analytics/churn-timeline?pastDays=0&futureDays=365")
	assert.Len(t, tl.Points, 365, "pastDays=0: forecast only")

	for path, msg := range map[string]string{
		"?pastDays=-1":    "pastDays: целое число от 0 до 365",
		"?pastDays=366":   "pastDays: целое число от 0 до 365",
		"?pastDays=week":  "pastDays: целое число от 0 до 365",
		"?futureDays=0":   "futureDays: целое число от 1 до 365",
		"?futureDays=366": "futureDays: целое число от 1 до 365",
		"?horizonDays=0":  "horizonDays: целое число от 1 до 365",
	} {
		rec := do(h, http.MethodGet, "/api/v1/analytics/churn-timeline"+path, "", cookie)
		assert.Equal(t, http.StatusBadRequest, rec.Code, path)
		assert.JSONEq(t, `{"error":{"code":"bad_request","message":"`+msg+`"}}`, rec.Body.String(), path)
	}
}

// horizonDays recalculates the probabilities like on the other screens; settings stay the same.
func TestChurnTimelineHorizon(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	sum := func(path string) float64 {
		_, tl := getJSON[timelineJSON](t, h, cookie, path)
		var s float64
		for _, p := range tl.Points {
			if p.ExpectedNonRenewals != nil {
				s += *p.ExpectedNonRenewals
			}
		}
		return s
	}
	base := sum("/api/v1/analytics/churn-timeline?pastDays=0")
	assert.Equal(t, base, sum("/api/v1/analytics/churn-timeline?pastDays=0&horizonDays=30"), "the saved horizon")
	assert.NotEqual(t, base, sum("/api/v1/analytics/churn-timeline?pastDays=0&horizonDays=7"))
}

func TestChurnTimelineStoreError(t *testing.T) {
	d := testDeps(t, nil)
	d.Timeline = &fakeTimeline{err: errors.New("connection refused")}
	h := NewRouter(d)
	rec := do(h, http.MethodGet, "/api/v1/analytics/churn-timeline", "", login(t, h))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "connection refused", "internals do not leak")
}
