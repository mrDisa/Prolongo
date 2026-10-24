package api

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

// JSON shapes of openapi.yml. Nullable fields are pointers without omitempty:
// the contract wants an explicit null (e.g. riskScore of a customer without a score).

type reasonJSON struct {
	Feature string  `json:"feature"`
	Points  float64 `json:"points"`
	Text    string  `json:"text"`
}

type customerListItemJSON struct {
	ID                    int64        `json:"id"`
	Email                 string       `json:"email"`
	BalanceCents          int64        `json:"balanceCents"`
	TotalReplenishedCents int64        `json:"totalReplenishedCents"`
	ScoreStatus           string       `json:"scoreStatus"`
	NoScoreReason         *string      `json:"noScoreReason"`
	RiskScore             *int         `json:"riskScore"`
	RiskLevel             *string      `json:"riskLevel"`
	Loyalty               *int         `json:"loyalty"`
	Probability           *float64     `json:"probability"`
	ExpectedLossCents     int64        `json:"expectedLossCents"`
	MRRCents              int64        `json:"mrrCents"`
	LiveServices          int          `json:"liveServices"`
	TopReasons            []reasonJSON `json:"topReasons"`
}

type customerListJSON struct {
	Items        []customerListItemJSON `json:"items"`
	Total        int                    `json:"total"`
	Page         int                    `json:"page"`
	PageSize     int                    `json:"pageSize"`
	CalcVersion  int64                  `json:"calcVersion"`
	CalculatedAt time.Time              `json:"calculatedAt"`
}

type moneyJSON struct {
	HorizonDays         int    `json:"horizonDays"`
	RevenueCents        int64  `json:"revenueCents"`
	BonusCoveredCents   int64  `json:"bonusCoveredCents"`
	BalanceCoveredCents int64  `json:"balanceCoveredCents"`
	AtRiskCents         int64  `json:"atRiskCents"`
	ExpectedLossCents   int64  `json:"expectedLossCents"`
	MRRCents            int64  `json:"mrrCents"`
	Explanation         string `json:"explanation"`
}

type contributionJSON struct {
	Feature   string  `json:"feature"`
	Title     string  `json:"title"`
	Scope     string  `json:"scope"`
	ServiceID *int64  `json:"serviceId"`
	Value     float64 `json:"value"`
	Weight    float64 `json:"weight"`
	Points    float64 `json:"points"`
	Text      string  `json:"text"`
}

type recommendationJSON struct {
	Feature string `json:"feature"`
	Action  string `json:"action"`
}

type serviceItemJSON struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	PaymentTerm string    `json:"paymentTerm"`
	AutoProlong bool      `json:"autoProlong"`
	PriceCents  int64     `json:"priceCents"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	DaysLeft    float64   `json:"daysLeft"`
	RiskScore   *int      `json:"riskScore"`
	MRRCents    int64     `json:"mrrCents"`
}

type transactionItemJSON struct {
	ID          int64      `json:"id"`
	ServiceID   int64      `json:"serviceId"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	AmountCents int64      `json:"amountCents"`
	CreatedAt   time.Time  `json:"createdAt"`
	PerformedAt *time.Time `json:"performedAt"`
}

type statusEventJSON struct {
	ID         int64     `json:"id"`
	ServiceID  int64     `json:"serviceId"`
	AccountID  int64     `json:"accountId"`
	FromStatus *string   `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	OccurredAt time.Time `json:"occurredAt"`
	Source     string    `json:"source"`
}

type customerDetailJSON struct {
	ID                    int64                 `json:"id"`
	Email                 string                `json:"email"`
	BalanceCents          int64                 `json:"balanceCents"`
	WithdrawBalanceCents  int64                 `json:"withdrawBalanceCents"`
	BonusBalanceCents     int64                 `json:"bonusBalanceCents"`
	BonusState            string                `json:"bonusState"`
	TotalReplenishedCents int64                 `json:"totalReplenishedCents"`
	ScoreStatus           string                `json:"scoreStatus"`
	NoScoreReason         *string               `json:"noScoreReason"`
	RiskScore             *int                  `json:"riskScore"`
	RiskLevel             *string               `json:"riskLevel"`
	Loyalty               *int                  `json:"loyalty"`
	Probability           *float64              `json:"probability"`
	Money                 moneyJSON             `json:"money"`
	RiskiestServiceID     *int64                `json:"riskiestServiceId"`
	Contributions         []contributionJSON    `json:"contributions"`
	Recommendations       []recommendationJSON  `json:"recommendations"`
	Services              []serviceItemJSON     `json:"services"`
	Transactions          []transactionItemJSON `json:"transactions"`
	StatusHistory         []statusEventJSON     `json:"statusHistory"`
	CalcVersion           int64                 `json:"calcVersion"`
	CalculatedAt          time.Time             `json:"calculatedAt"`
}

type summaryJSON struct {
	CalcVersion   int64     `json:"calcVersion"`
	CalculatedAt  time.Time `json:"calculatedAt"`
	ReferenceTime time.Time `json:"referenceTime"`
	ClockMode     string    `json:"clockMode"`
	HorizonDays   int       `json:"horizonDays"`
	Customers     struct {
		Total   int `json:"total"`
		High    int `json:"high"`
		Medium  int `json:"medium"`
		Low     int `json:"low"`
		NoScore int `json:"noScore"`
	} `json:"customers"`
	Services struct {
		Total       int `json:"total"`
		Live        int `json:"live"`
		AtRisk      int `json:"atRisk"`
		ProlongWait int `json:"prolongWait"`
	} `json:"services"`
	Money struct {
		ExpectedLossCents  int64 `json:"expectedLossCents"`
		WorstCaseLossCents int64 `json:"worstCaseLossCents"`
		MRRAtRiskCents     int64 `json:"mrrAtRiskCents"`
		TotalMRRCents      int64 `json:"totalMrrCents"`
		SavableTop20Cents  int64 `json:"savableTop20Cents"`
	} `json:"money"`
	TopAtRisk []customerListItemJSON `json:"topAtRisk"`
}

// roundTo keeps n decimals: points to 0.1 (as in the UI), probability to 0.0001.
func roundTo(x float64, n int) float64 {
	p := math.Pow10(n)
	return math.Round(x*p) / p
}

// scoreFields fills the fields that are null for customers without a score.
func scoreFields(cr *engine.CustomerResult) (reason *string, score *int, level *string, loyalty *int, probability *float64) {
	if cr.ScoreStatus != engine.StatusScored {
		r := cr.NoScoreReason
		return &r, nil, nil, nil, nil
	}
	s, l, lo, p := cr.RiskScore, cr.Level, cr.Loyalty, roundTo(cr.Probability, 4)
	return nil, &s, &l, &lo, &p
}

func toListItem(cr *engine.CustomerResult) customerListItemJSON {
	item := customerListItemJSON{
		ID: cr.Account.ID, Email: cr.Account.Email,
		BalanceCents: cr.Account.Balance, TotalReplenishedCents: cr.Account.TotalReplenished,
		ScoreStatus: cr.ScoreStatus, ExpectedLossCents: cr.Money.ExpectedLoss, MRRCents: cr.Money.MRR,
		LiveServices: cr.LiveServices, TopReasons: []reasonJSON{},
	}
	item.NoScoreReason, item.RiskScore, item.RiskLevel, item.Loyalty, item.Probability = scoreFields(cr)
	for _, r := range cr.TopReasons {
		item.TopReasons = append(item.TopReasons, reasonJSON{Feature: r.Feature, Points: roundTo(r.Points, 1), Text: r.Text})
	}
	return item
}

func toMoneyJSON(m engine.Money) moneyJSON {
	return moneyJSON{
		HorizonDays: m.HorizonDays, RevenueCents: m.Revenue, BonusCoveredCents: m.BonusCovered,
		BalanceCoveredCents: m.BalanceCovered, AtRiskCents: m.AtRisk, ExpectedLossCents: m.ExpectedLoss,
		MRRCents: m.MRR, Explanation: m.Explanation,
	}
}

// toDetail builds the customer card. txByService and history come from the snapshot's
// dataset and from public.status_events.
func toDetail(cr *engine.CustomerResult, txByService map[int64][]domain.Transaction, history []store.StatusEvent) customerDetailJSON {
	d := customerDetailJSON{
		ID: cr.Account.ID, Email: cr.Account.Email, BalanceCents: cr.Account.Balance,
		WithdrawBalanceCents: cr.Account.WithdrawBalance, BonusBalanceCents: cr.Account.BonusBalance,
		BonusState: "locked", TotalReplenishedCents: cr.Account.TotalReplenished,
		ScoreStatus: cr.ScoreStatus, Money: toMoneyJSON(cr.Money),
		Contributions: []contributionJSON{}, Recommendations: []recommendationJSON{},
		Services: []serviceItemJSON{}, Transactions: []transactionItemJSON{}, StatusHistory: []statusEventJSON{},
	}
	if cr.Account.BonusUnlocked {
		d.BonusState = "unlocked"
	}
	d.NoScoreReason, d.RiskScore, d.RiskLevel, d.Loyalty, d.Probability = scoreFields(cr)
	if cr.RiskiestServiceID != 0 {
		id := cr.RiskiestServiceID
		d.RiskiestServiceID = &id
	}
	for _, c := range cr.Contributions {
		cj := contributionJSON{Feature: c.Feature, Title: c.Title, Scope: string(c.Scope),
			Value: roundTo(c.Value, 3), Weight: c.Weight, Points: roundTo(c.Points, 1), Text: c.Text}
		if c.ServiceID != 0 {
			id := c.ServiceID
			cj.ServiceID = &id
		}
		d.Contributions = append(d.Contributions, cj)
	}
	for _, r := range cr.Recommendations {
		d.Recommendations = append(d.Recommendations, recommendationJSON{Feature: r.Feature, Action: r.Action})
	}
	for _, sr := range cr.Services {
		s := sr.Service
		item := serviceItemJSON{ID: s.ID, Name: s.Name, Status: string(s.Status), PaymentTerm: string(s.PaymentTerm),
			AutoProlong: s.AutoProlong, PriceCents: s.Price, CreatedAt: s.CreatedAt, ExpiresAt: s.ExpiresAt,
			DaysLeft: roundTo(sr.DaysLeft, 1), MRRCents: sr.MRR}
		if sr.Scored {
			score := sr.Score
			item.RiskScore = &score
		}
		d.Services = append(d.Services, item)
		for _, t := range txByService[s.ID] {
			status := "not_performed"
			if t.Performed {
				status = "performed"
			}
			d.Transactions = append(d.Transactions, transactionItemJSON{ID: t.ID, ServiceID: t.ServiceID, Type: string(t.Type),
				Status: status, AmountCents: t.Amount, CreatedAt: t.CreatedAt, PerformedAt: t.PerformedAt})
		}
	}
	// Newest payments first; equal times keep a stable order by id.
	slices.SortStableFunc(d.Transactions, func(a, b transactionItemJSON) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	for _, e := range history {
		d.StatusHistory = append(d.StatusHistory, statusEventJSON{ID: e.ID, ServiceID: e.ServiceID, AccountID: e.AccountID,
			FromStatus: e.FromStatus, ToStatus: e.ToStatus, OccurredAt: e.OccurredAt, Source: e.Source})
	}
	return d
}
