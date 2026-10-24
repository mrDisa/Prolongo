package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
)

// TimelineSource reads the facts of the churn chart (*store.Store).
type TimelineSource interface {
	TimelineFacts(ctx context.Context, from, to time.Time) (engine.TimelineFacts, error)
}

// timelineJSON is the TimelineResponse schema of openapi.yml.
type timelineJSON struct {
	ReferenceTime time.Time           `json:"referenceTime"`
	Points        []timelinePointJSON `json:"points"`
}

// timelinePointJSON: fact points carry deletions, forecast points carry expectedNonRenewals;
// servicesAtRisk is in both, null when unknown (docs/ОБЩЕЕ_ТЗ.md §7.7).
type timelinePointJSON struct {
	Date                string   `json:"date"`
	Kind                string   `json:"kind"`
	Deletions           *int     `json:"deletions,omitempty"`
	DeletionsEstimated  *int     `json:"deletionsEstimated,omitempty"`
	ServicesAtRisk      *int     `json:"servicesAtRisk"`
	ExpectedNonRenewals *float64 `json:"expectedNonRenewals,omitempty"`
}

// churnTimeline: GET /api/v1/analytics/churn-timeline — the fact / forecast chart of the overview.
func (h *handlers) churnTimeline(w http.ResponseWriter, r *http.Request) {
	past, err := queryInt(r, "pastDays", 90, 0, 365)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error(), nil)
		return
	}
	future, err := queryInt(r, "futureDays", 30, 1, 365)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error(), nil)
		return
	}
	_, res, ok := h.snapshotAndResult(w, r) // horizonDays changes the probabilities of the forecast
	if !ok {
		return
	}

	from, to := engine.TimelineRange(res.Now, past)
	facts, err := h.timeline.TimelineFacts(r.Context(), from, to)
	if err != nil {
		writeInternal(w, r, h.logger, err)
		return
	}

	points := res.Timeline(facts, past, future)
	out := timelineJSON{ReferenceTime: res.Now, Points: make([]timelinePointJSON, len(points))}
	for i, p := range points {
		j := timelinePointJSON{Date: p.Day, Kind: p.Kind}
		if p.Kind == engine.TimelineFact {
			j.Deletions, j.DeletionsEstimated, j.ServicesAtRisk = &p.Deletions, &p.DeletionsEstimated, p.ServicesAtRisk
		} else {
			v := math.Round(p.ExpectedNonRenewals*100) / 100
			j.ExpectedNonRenewals = &v
		}
		out.Points[i] = j
	}
	writeJSON(w, http.StatusOK, out)
}

// queryInt reads an optional integer query parameter within [lo, hi]; the error names it for the 400.
func queryInt(r *http.Request, name string, def, lo, hi int) (int, error) {
	n, err := intParam(r.URL.Query().Get(name), def, lo, hi)
	if err != nil {
		return 0, fmt.Errorf("%s: целое число от %d до %d", name, lo, hi)
	}
	return n, nil
}
