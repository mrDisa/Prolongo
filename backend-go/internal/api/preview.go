package api

import (
	"cmp"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

// PreviewResponse of openapi.yml.

type previewCountsJSON struct {
	Customers struct {
		High    int `json:"high"`
		Medium  int `json:"medium"`
		Low     int `json:"low"`
		NoScore int `json:"noScore"`
	} `json:"customers"`
	Money struct {
		ExpectedLossCents  int64 `json:"expectedLossCents"`
		WorstCaseLossCents int64 `json:"worstCaseLossCents"`
	} `json:"money"`
}

type previewPointsJSON struct {
	Feature string  `json:"feature"`
	Points  float64 `json:"points"`
}

type previewCustomerJSON struct {
	ID                  int64               `json:"id"`
	Email               string              `json:"email"`
	RiskScore           int                 `json:"riskScore"`
	RiskScoreRaw        float64             `json:"riskScoreRaw"`
	RiskLevel           string              `json:"riskLevel"`
	ExpectedLossCents   int64               `json:"expectedLossCents"`
	RiskiestServiceName string              `json:"riskiestServiceName"`
	Contributions       []previewPointsJSON `json:"contributions"`
}

type previewFeatureJSON struct {
	Key          string  `json:"key"`
	CustomersHit int     `json:"customersHit"`
	MaxPoints    float64 `json:"maxPoints"`
}

type previewJSON struct {
	Summary   previewCountsJSON     `json:"summary"`
	Baseline  previewCountsJSON     `json:"baseline"`
	Customers []previewCustomerJSON `json:"customers"`
	Features  []previewFeatureJSON  `json:"features"`
}

// preview: POST /api/v1/model/preview — the draft (same body as PATCH) on top of the saved
// settings, calculated on the same data. Nothing is saved and no SSE is sent: the "Model"
// page calls it on every slider move.
func (h *handlers) preview(w http.ResponseWriter, r *http.Request) {
	p, ok := decodePatch(w, r)
	if !ok {
		return
	}
	snap, ok := h.snapshot(w, r)
	if !ok {
		return
	}
	draft, errs := engine.Apply(snap.Settings, p)
	if len(errs) > 0 {
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Настройки не прошли проверку", errs)
		return
	}
	now := snap.Now
	if draft.Clock != snap.Settings.Clock {
		now = engine.Now(draft.Clock, snap.Dataset, time.Now())
	}
	res := engine.Calculate(snap.Dataset, draft, now)

	out := previewJSON{
		Summary:   previewCounts(res.Summary),
		Baseline:  previewCounts(snap.Summary),
		Customers: []previewCustomerJSON{},
	}
	type hit struct {
		customers int
		maxPoints float64 // the strongest contribution, sign kept
	}
	hits := map[string]*hit{}
	for i := range res.Customers {
		cr := &res.Customers[i]
		if cr.ScoreStatus != engine.StatusScored {
			continue
		}
		pc := previewCustomerJSON{
			ID: cr.Account.ID, Email: cr.Account.Email, RiskScore: cr.RiskScore, RiskScoreRaw: roundTo(cr.RiskScoreRaw, 1),
			RiskLevel: cr.Level, ExpectedLossCents: cr.Money.ExpectedLoss,
			RiskiestServiceName: serviceName(cr), Contributions: []previewPointsJSON{},
		}
		for _, c := range cr.Contributions {
			pc.Contributions = append(pc.Contributions, previewPointsJSON{Feature: c.Feature, Points: roundTo(c.Points, 1)})
			hh := hits[c.Feature]
			if hh == nil {
				hh = &hit{}
				hits[c.Feature] = hh
			}
			hh.customers++
			if math.Abs(c.Points) > math.Abs(hh.maxPoints) {
				hh.maxPoints = c.Points
			}
		}
		out.Customers = append(out.Customers, pc)
	}
	// Riskiest first: that is how the scale on the "Model" page reads them.
	slices.SortStableFunc(out.Customers, func(a, b previewCustomerJSON) int {
		if c := cmp.Compare(b.RiskScoreRaw, a.RiskScoreRaw); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	// Every registered feature, including the ones that hit nobody with this draft:
	// the UI shows "сейчас срабатывает у N клиентов" next to each slider.
	for _, f := range features.All() {
		key := f.Meta().Key
		fj := previewFeatureJSON{Key: key}
		if hh := hits[key]; hh != nil {
			fj.CustomersHit, fj.MaxPoints = hh.customers, roundTo(hh.maxPoints, 1)
		}
		out.Features = append(out.Features, fj)
	}
	writeJSON(w, http.StatusOK, out)
}

func previewCounts(s engine.Summary) previewCountsJSON {
	var c previewCountsJSON
	c.Customers.High, c.Customers.Medium, c.Customers.Low, c.Customers.NoScore =
		s.Customers.High, s.Customers.Medium, s.Customers.Low, s.Customers.NoScore
	c.Money.ExpectedLossCents, c.Money.WorstCaseLossCents = s.Money.ExpectedLoss, s.Money.WorstCase
	return c
}

func serviceName(cr *engine.CustomerResult) string {
	for _, s := range cr.Services {
		if s.Service.ID == cr.RiskiestServiceID {
			return s.Service.Name
		}
	}
	return ""
}
