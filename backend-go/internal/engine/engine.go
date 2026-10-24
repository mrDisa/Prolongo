// Package engine turns a Dataset and Settings into risk scores and money (docs/ТЗ_BACKEND_GO.md §9).
package engine

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

const (
	topReasonsCount     = 3
	recommendMinPoints  = 5 // actions only for features adding at least 5 points
	topAtRiskCount      = 10
	savableCustomersTop = 20
)

// Calculate scores every account. It only reads ds and settings, so it is safe to run
// in parallel with other calculations (G8.1 previews other horizons this way).
func Calculate(ds *domain.Dataset, settings Settings, now time.Time) *Result {
	r := &Result{
		Now:         now,
		ClockMode:   settings.Clock.Mode,
		HorizonDays: settings.HorizonDays,
		Settings:    settings,
		Customers:   make([]CustomerResult, 0, len(ds.Accounts)),
		index:       make(map[int64]int, len(ds.Accounts)),
	}
	feats := enabledFeatures(settings)
	for _, acc := range ds.Accounts {
		c := features.NewContext(acc, ds.ServicesByAccount[acc.ID], ds.TxByService, now, float64(settings.HorizonDays))
		r.index[acc.ID] = len(r.Customers)
		r.Customers = append(r.Customers, scoreAccount(c, feats, settings))
	}
	r.Summary = summarize(r.Customers, settings)
	return r
}

// weighted is a feature with its effective weight.
type weighted struct {
	f      features.Feature
	meta   features.Meta
	weight float64
}

func enabledFeatures(s Settings) []weighted {
	var out []weighted
	for _, f := range features.All() {
		m := f.Meta()
		if fs := s.feature(m); fs.Enabled && fs.Weight != 0 {
			out = append(out, weighted{f: f, meta: m, weight: fs.Weight})
		}
	}
	return out
}

func scoreAccount(c *features.Context, feats []weighted, settings Settings) CustomerResult {
	cr := CustomerResult{Account: c.Account, LiveServices: len(c.Live), Money: Money{HorizonDays: settings.HorizonDays, MRR: c.MRR}}

	// 1. No live services → no score, with the reason.
	if reason := noScoreReason(c.Services); reason != "" {
		cr.ScoreStatus, cr.NoScoreReason = StatusNoScore, reason
		cr.Money.Explanation = "Живых услуг нет — денежной оценки нет."
		cr.Services = serviceResults(c, nil)
		return cr
	}

	// 2. Account features are the same for every service: compute them once.
	var accountContribs []Contribution
	for _, w := range feats {
		if w.meta.Scope == features.ScopeAccount {
			if ct, ok := contribute(w, c, nil); ok {
				accountContribs = append(accountContribs, ct)
			}
		}
	}

	// 3. Each live service: its own features + account features → score(s) = clamp(Σ points, 0, 100).
	scores := make(map[int64]float64, len(c.Live))
	var best []Contribution
	bestRaw, bestID := -1.0, int64(0)
	for i := range c.Live {
		s := &c.Live[i]
		contribs := slices.Clone(accountContribs)
		for _, w := range feats {
			if w.meta.Scope == features.ScopeService {
				if ct, ok := contribute(w, c, s); ok {
					contribs = append(contribs, ct)
				}
			}
		}
		var sum float64
		for _, ct := range contribs {
			sum += ct.Points
		}
		raw := features.Clamp(sum, 0, 100)
		scores[s.ID] = raw
		// 4. The account score is the riskiest service: attention is needed even if one of five burns.
		if raw > bestRaw {
			bestRaw, bestID, best = raw, s.ID, contribs
		}
	}

	cr.ScoreStatus = StatusScored
	cr.RiskScoreRaw = bestRaw
	cr.RiskScore = int(math.Round(bestRaw))
	cr.Level = settings.level(cr.RiskScore)
	cr.Loyalty = 100 - cr.RiskScore
	cr.Probability = bestRaw / 100
	cr.RiskiestServiceID = bestID

	slices.SortStableFunc(best, func(a, b Contribution) int {
		if c := cmp.Compare(math.Abs(b.Points), math.Abs(a.Points)); c != 0 {
			return c
		}
		return cmp.Compare(a.Feature, b.Feature)
	})
	cr.Contributions = best
	cr.TopReasons, cr.Recommendations = reasonsAndActions(best, feats)
	cr.Money = money(c, cr.Probability, settings.HorizonDays)
	cr.Services = serviceResults(c, scores)
	return cr
}

// noScoreReason: no services → no_services; none live and all blocked → blocked; none live otherwise → churned.
func noScoreReason(services []domain.Service) string {
	if len(services) == 0 {
		return NoScoreNoServices
	}
	allBlocked := true
	for _, s := range services {
		if s.Live() {
			return ""
		}
		if s.Status != domain.StatusBlocked {
			allBlocked = false
		}
	}
	if allBlocked {
		return NoScoreBlocked
	}
	return NoScoreChurned
}

// contribute computes one feature; ok is false when it gives no points.
func contribute(w weighted, c *features.Context, s *domain.Service) (Contribution, bool) {
	value, text := w.f.Compute(c, s)
	points := value * w.weight
	if points == 0 {
		return Contribution{}, false
	}
	ct := Contribution{Feature: w.meta.Key, Title: w.meta.Title, Scope: w.meta.Scope, Value: value, Weight: w.weight, Points: points, Text: text}
	if s != nil {
		ct.ServiceID = s.ID
	}
	return ct, true
}

// reasonsAndActions: top 3 positive contributions; actions of features with ≥ 5 points, by points desc.
func reasonsAndActions(sorted []Contribution, feats []weighted) ([]Contribution, []Recommendation) {
	actions := make(map[string]string, len(feats))
	for _, w := range feats {
		actions[w.meta.Key] = w.meta.Action
	}
	var top []Contribution
	var recs []Recommendation
	for _, ct := range sorted { // already by |points| desc; positive ones keep their order
		if ct.Points <= 0 {
			continue
		}
		if len(top) < topReasonsCount {
			top = append(top, ct)
		}
		if ct.Points >= recommendMinPoints && actions[ct.Feature] != "" {
			recs = append(recs, Recommendation{Feature: ct.Feature, Action: actions[ct.Feature]})
		}
	}
	return top, recs
}

// serviceResults lists every service of the account, riskiest first, then the non-live ones.
func serviceResults(c *features.Context, scores map[int64]float64) []ServiceResult {
	out := make([]ServiceResult, 0, len(c.Services))
	for _, s := range c.Services {
		sr := ServiceResult{Service: s, DaysLeft: features.DaysLeft(s, c.Now)}
		if raw, ok := scores[s.ID]; ok {
			sr.Scored, sr.ScoreRaw, sr.Score, sr.MRR = true, raw, int(math.Round(raw)), features.MRR(s)
		}
		out = append(out, sr)
	}
	slices.SortStableFunc(out, func(a, b ServiceResult) int {
		if a.Scored != b.Scored {
			if a.Scored {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(b.ScoreRaw, a.ScoreRaw); c != 0 {
			return c
		}
		return cmp.Compare(a.Service.ID, b.Service.ID)
	})
	return out
}

// summarize builds the overview numbers (docs/ОБЩЕЕ_ТЗ.md §5, §7.6).
func summarize(customers []CustomerResult, settings Settings) Summary {
	var s Summary
	s.Customers.Total = len(customers)
	var scored []*CustomerResult
	for i := range customers {
		cr := &customers[i]
		for _, sr := range cr.Services {
			s.Services.Total++
			if sr.Service.Live() {
				s.Services.Live++
			}
			if sr.Service.Status == domain.StatusProlongWait {
				s.Services.ProlongWait++
			}
			if sr.Scored && sr.Score >= settings.Thresholds.Medium {
				s.Services.AtRisk++
			}
		}
		if cr.ScoreStatus != StatusScored {
			s.Customers.NoScore++
			continue
		}
		scored = append(scored, cr)
		s.Money.ExpectedLoss += cr.Money.ExpectedLoss
		s.Money.TotalMRR += cr.Money.MRR
		switch cr.Level {
		case LevelHigh:
			s.Customers.High++
			s.Money.WorstCase += cr.Money.AtRisk
			s.Money.MRRAtRisk += cr.Money.MRR
		case LevelMedium:
			s.Customers.Medium++
			s.Money.MRRAtRisk += cr.Money.MRR
		default:
			s.Customers.Low++
		}
	}

	slices.SortStableFunc(scored, func(a, b *CustomerResult) int {
		if c := cmp.Compare(b.Money.ExpectedLoss, a.Money.ExpectedLoss); c != 0 {
			return c
		}
		return cmp.Compare(a.Account.ID, b.Account.ID)
	})
	for i, cr := range scored {
		if i < savableCustomersTop {
			s.Money.SavableTop20 += cr.Money.ExpectedLoss
		}
		if i < topAtRiskCount {
			s.TopAtRisk = append(s.TopAtRisk, cr.Account.ID)
		}
	}
	return s
}
