package engine

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

// referenceNow is the data clock on the organizers' data: the latest timestamp in data.json.
var referenceNow = time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC)

func reference(t *testing.T) *domain.Dataset {
	t.Helper()
	f, err := os.Open("../../../data/test-data/data.json")
	require.NoError(t, err)
	defer f.Close()
	ds, err := domain.FromJSON(f)
	require.NoError(t, err)
	return ds
}

func calc(t *testing.T, mutate func(*Settings)) *Result {
	t.Helper()
	ds := reference(t)
	s := DefaultSettings()
	if mutate != nil {
		mutate(&s)
	}
	return Calculate(ds, s, Now(s.Clock, ds, time.Now()))
}

func customer(t *testing.T, r *Result, id int64) *CustomerResult {
	t.Helper()
	cr, ok := r.Customer(id)
	require.True(t, ok, "customer %d", id)
	return cr
}

// The acceptance table of docs/ОБЩЕЕ_ТЗ.md §4.2.1.
func TestReferenceScores(t *testing.T) {
	r := calc(t, nil)
	assert.Equal(t, referenceNow, r.Now)

	scored := []struct {
		id           int64
		score        int
		level        string
		riskiest     int64
		expectedLoss int64
	}{
		{5, 77, LevelHigh, 25, 338372},
		{9, 66, LevelHigh, 9, 343971},
		{10, 41, LevelMedium, 20, 0}, // 40.5 rounds up, as math.Round does
		{1, 31, LevelMedium, 1, 20924},
		{4, 4, LevelLow, 14, 0},
		{6, 0, LevelLow, 6, 0},
	}
	for _, want := range scored {
		cr := customer(t, r, want.id)
		assert.Equal(t, StatusScored, cr.ScoreStatus, "#%d", want.id)
		assert.Equal(t, want.score, cr.RiskScore, "#%d score", want.id)
		assert.Equal(t, want.level, cr.Level, "#%d level", want.id)
		assert.Equal(t, want.riskiest, cr.RiskiestServiceID, "#%d riskiest service", want.id)
		assert.InDelta(t, want.expectedLoss, cr.Money.ExpectedLoss, 1, "#%d expected loss", want.id)
		assert.Equal(t, 100-cr.RiskScore, cr.Loyalty)
		assert.InDelta(t, cr.RiskScoreRaw/100, cr.Probability, 1e-12)
	}

	noScore := map[int64]string{2: NoScoreBlocked, 7: NoScoreBlocked, 3: NoScoreChurned, 8: NoScoreChurned}
	for id, reason := range noScore {
		cr := customer(t, r, id)
		assert.Equal(t, StatusNoScore, cr.ScoreStatus, "#%d", id)
		assert.Equal(t, reason, cr.NoScoreReason, "#%d", id)
		assert.Empty(t, cr.Level)
		assert.Empty(t, cr.Contributions)
		assert.Zero(t, cr.Money.ExpectedLoss)
	}
}

func TestReferenceCustomer5(t *testing.T) {
	cr := customer(t, calc(t, nil), 5)

	type pts struct {
		feature string
		points  float64
	}
	var got []pts
	for _, c := range cr.Contributions {
		got = append(got, pts{c.Feature, math.Round(c.Points*10) / 10})
	}
	assert.Equal(t, []pts{
		{"prolong_wait", 45}, {"insufficient_funds", 28.2}, {"loyal_payer", -10},
		{"new_client", 5.5}, {"short_billing", 5}, {"failed_payments", 3},
	}, got, "by |points| desc")
	assert.InDelta(t, 76.71, cr.RiskScoreRaw, 0.01)

	require.Len(t, cr.TopReasons, 3)
	assert.Equal(t, "service-0025: срок ожидания оплаты истёк 395 дн. назад — удаление в любой момент", cr.TopReasons[0].Text)
	assert.Equal(t, "На балансе €286,57, на продления за 30 дн. нужно €4 697,65", cr.TopReasons[1].Text)
	assert.Equal(t, "new_client", cr.TopReasons[2].Feature, "loyal_payer is negative: not a reason")

	var actions []string
	for _, r := range cr.Recommendations {
		actions = append(actions, r.Feature)
	}
	assert.Equal(t, []string{"prolong_wait", "insufficient_funds", "new_client", "short_billing"}, actions, "≥ 5 points only")

	assert.Equal(t, Money{
		HorizonDays: 30, Revenue: 469765, BonusCovered: 0, BalanceCovered: 28657, AtRisk: 441108, ExpectedLoss: 338372, MRR: 469140,
		Explanation: "Услуги принесут €4 697,65 за 30 дней. Бонусы заблокированы, €286,57 спишется с баланса автоматически. " +
			"Под риском €4 411,08, вероятность ухода 77% → ожидаемые потери €3 383,72.",
	}, cr.Money)

	require.Len(t, cr.Services, 3)
	assert.Equal(t, []int64{25, 5, 15}, []int64{cr.Services[0].Service.ID, cr.Services[1].Service.ID, cr.Services[2].Service.ID}, "riskiest first")
	assert.Equal(t, 77, cr.Services[0].Score)
	assert.EqualValues(t, 468720, cr.Services[0].MRR)
	assert.Equal(t, 3, cr.LiveServices)
}

func TestReferenceSummary(t *testing.T) {
	s := calc(t, nil).Summary
	assert.Equal(t, [5]int{10, 2, 2, 2, 4}, [5]int{s.Customers.Total, s.Customers.High, s.Customers.Medium, s.Customers.Low, s.Customers.NoScore})
	assert.Equal(t, [4]int{30, 18, 10, 6}, [4]int{s.Services.Total, s.Services.Live, s.Services.AtRisk, s.Services.ProlongWait})
	assert.InDelta(t, 703267, s.Money.ExpectedLoss, 2, "€7 032,67")
	assert.EqualValues(t, 966052, s.Money.WorstCase, "€9 660,52")
	assert.EqualValues(t, 1155577, s.Money.MRRAtRisk, "€11 555,77")
	assert.EqualValues(t, 1159275, s.Money.TotalMRR, "€11 592,75")
	assert.Equal(t, s.Money.ExpectedLoss, s.Money.SavableTop20, "fewer than 20 customers: everything is savable")
	assert.Equal(t, []int64{9, 5, 1, 4, 6, 10}, s.TopAtRisk, "by expected loss desc, then id")
}

// Expected loss for the period switcher on the overview (G8.1 checks horizonDays=7 → 89 738).
func TestReferenceHorizons(t *testing.T) {
	cases := map[int]int64{7: 89738, 14: 271258, 30: 703267, 60: 1537758, 90: 2378403}
	for h, want := range cases {
		r := calc(t, func(s *Settings) { s.HorizonDays = h })
		assert.InDelta(t, want, r.Summary.Money.ExpectedLoss, 2, "H=%d", h)
		assert.Equal(t, h, r.HorizonDays)
	}
	r7 := calc(t, func(s *Settings) { s.HorizonDays = 7 }).Summary
	assert.Equal(t, [3]int{1, 3, 2}, [3]int{r7.Customers.High, r7.Customers.Medium, r7.Customers.Low})
}

// The worked example of docs/ОБЩЕЕ_ТЗ.md §5: €10 revenue, €2 bonuses, €3 from the balance, p = 0.72 → €3.60.
func TestMoneyWorkedExample(t *testing.T) {
	s := domain.Service{ID: 1, Name: "service-0001", OwnerID: 1, PaymentTerm: domain.TermMonth, AutoProlong: true,
		Status: domain.StatusActive, Price: 1000, ExpiresAt: referenceNow.AddDate(0, 0, 10)}
	acc := domain.Account{ID: 1, Balance: 300, BonusBalance: 200, BonusUnlocked: true}
	c := features.NewContext(acc, []domain.Service{s}, nil, referenceNow, 30)

	m := money(c, 0.72, 30)
	assert.Equal(t, Money{
		HorizonDays: 30, Revenue: 1000, BonusCovered: 200, BalanceCovered: 300, AtRisk: 500, ExpectedLoss: 360, MRR: 1000,
		Explanation: "Услуги принесут €10,00 за 30 дней. €2,00 покроют бонусы, €3,00 спишется с баланса автоматически. " +
			"Под риском €5,00, вероятность ухода 72% → ожидаемые потери €3,60.",
	}, m)
}

func TestMoneyFullyCovered(t *testing.T) {
	cr := customer(t, calc(t, nil), 10) // bonuses €55,14 > €21,71 due
	assert.Zero(t, cr.Money.AtRisk)
	assert.Zero(t, cr.Money.ExpectedLoss)
	assert.Equal(t, cr.Money.Revenue, cr.Money.BonusCovered+cr.Money.BalanceCovered, "parts add up to revenue")
	assert.Contains(t, cr.Money.Explanation, "потерь не ожидается")
}

func TestWeightsChangeScores(t *testing.T) {
	base := customer(t, calc(t, nil), 5).RiskScoreRaw

	heavier := calc(t, func(s *Settings) { s.Features["prolong_wait"] = FeatureSetting{Weight: 50, Enabled: true} })
	assert.InDelta(t, base+5, customer(t, heavier, 5).RiskScoreRaw, 1e-9)

	off := calc(t, func(s *Settings) { s.Features["insufficient_funds"] = FeatureSetting{Weight: 30, Enabled: false} })
	cr := customer(t, off, 5)
	assert.InDelta(t, base-30*(1-28657.0/469765), cr.RiskScoreRaw, 1e-9)
	for _, c := range cr.Contributions {
		assert.NotEqual(t, "insufficient_funds", c.Feature, "a disabled feature gives nothing")
	}
}

func TestMissingFeatureSettingUsesDefaults(t *testing.T) {
	// Saved settings from before a feature existed: it must still work at its default weight.
	r := calc(t, func(s *Settings) { delete(s.Features, "prolong_wait") })
	assert.Equal(t, 77, customer(t, r, 5).RiskScore)
}

func TestThresholds(t *testing.T) {
	s := calc(t, func(s *Settings) { s.Thresholds.High = 40 }).Summary
	assert.Equal(t, 3, s.Customers.High, "G8.1: preview with high=40 → 3 high")
	assert.Equal(t, 1, s.Customers.Medium)
}

func TestScoreIsClamped(t *testing.T) {
	r := calc(t, func(s *Settings) { s.Features["prolong_wait"] = FeatureSetting{Weight: 100, Enabled: true} })
	assert.Equal(t, 100, customer(t, r, 5).RiskScore, "never above 100")
	r = calc(t, func(s *Settings) { s.Features["loyal_payer"] = FeatureSetting{Weight: -50, Enabled: true} })
	assert.Equal(t, 0, customer(t, r, 4).RiskScore, "never below 0")
}

func TestNoServices(t *testing.T) {
	ds := &domain.Dataset{Accounts: []domain.Account{{ID: 1}}, ServicesByAccount: map[int64][]domain.Service{}}
	r := Calculate(ds, DefaultSettings(), referenceNow)
	cr := customer(t, r, 1)
	assert.Equal(t, NoScoreNoServices, cr.NoScoreReason)
	assert.Equal(t, 1, r.Summary.Customers.NoScore)
}

func TestNow(t *testing.T) {
	real := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	fixed := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	source := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

	ds := &domain.Dataset{LatestTimestamp: referenceNow}
	assert.Equal(t, referenceNow, Now(Clock{Mode: ClockData}, ds, real), "data without source clock → latest timestamp")
	ds.SourceTime = &source
	assert.Equal(t, source, Now(Clock{Mode: ClockData}, ds, real), "source clock wins")
	assert.Equal(t, real, Now(Clock{Mode: ClockData}, &domain.Dataset{}, real), "empty data → real time")
	assert.Equal(t, real, Now(Clock{Mode: ClockReal}, ds, real))
	assert.Equal(t, fixed, Now(Clock{Mode: ClockFixed, FixedAt: &fixed}, ds, real))
	assert.Equal(t, real, Now(Clock{Mode: ClockFixed}, ds, real), "fixed without a date → real time")
}

func TestCapitalize(t *testing.T) {
	assert.Equal(t, "Бонусы заблокированы", capitalize("бонусы заблокированы"))
	assert.Equal(t, "€2,00 покроют бонусы", capitalize("€2,00 покроют бонусы"))
}
