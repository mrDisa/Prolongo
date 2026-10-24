package engine

import (
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

// Score statuses, no-score reasons and levels from the API contract.
const (
	StatusScored  = "scored"
	StatusNoScore = "no_score"

	NoScoreNoServices = "no_services"
	NoScoreChurned    = "churned"
	NoScoreBlocked    = "blocked"

	LevelHigh   = "high"
	LevelMedium = "medium"
	LevelLow    = "low"
)

// Result is one full calculation. It is never modified after Calculate returns,
// so many HTTP requests can read the same Result at once without locks.
type Result struct {
	Now         time.Time
	ClockMode   string
	HorizonDays int
	Settings    Settings

	Customers []CustomerResult // ordered by account id
	Summary   Summary

	index map[int64]int // account id → position in Customers
}

// Customer returns the result of one account.
func (r *Result) Customer(id int64) (*CustomerResult, bool) {
	i, ok := r.index[id]
	if !ok {
		return nil, false
	}
	return &r.Customers[i], true
}

// CustomerResult is the assessment of one account.
type CustomerResult struct {
	Account       domain.Account
	ScoreStatus   string // scored | no_score
	NoScoreReason string // no_services | churned | blocked; empty when scored

	RiskScore    int     // rounded 0..100: shown to the user and used for the level
	RiskScoreRaw float64 // unrounded: used for probability and money
	Level        string  // high | medium | low; empty when no_score
	Loyalty      int     // 100 − RiskScore
	Probability  float64 // RiskScoreRaw / 100

	Money             Money
	RiskiestServiceID int64 // 0 when no_score

	Contributions   []Contribution // non-zero features of the riskiest service, by |points| desc
	TopReasons      []Contribution // up to 3 contributions with points > 0
	Recommendations []Recommendation

	Services     []ServiceResult // all services, riskiest first
	LiveServices int
}

// Contribution is one feature's share of a service score.
type Contribution struct {
	Feature   string
	Title     string
	Scope     features.Scope
	ServiceID int64 // 0 for account features
	Value     float64
	Weight    float64
	Points    float64 // Value × Weight, unrounded
	Text      string
}

// Recommendation is an action for the marketer.
type Recommendation struct {
	Feature string
	Action  string
}

// ServiceResult is one service with its own score.
type ServiceResult struct {
	Service  domain.Service
	DaysLeft float64
	Scored   bool // live services only
	Score    int
	ScoreRaw float64
	MRR      int64
}

// Money is the financial estimate of an account for the horizon, in cents (docs/ОБЩЕЕ_ТЗ.md §5).
type Money struct {
	HorizonDays    int
	Revenue        int64 // what live services would bring within the horizon
	BonusCovered   int64 // paid by bonuses (only if unlocked)
	BalanceCovered int64 // charged automatically from the balance (auto-renewing services)
	AtRisk         int64 // Revenue − BonusCovered − BalanceCovered
	ExpectedLoss   int64 // round(Probability × AtRisk)
	MRR            int64
	Explanation    string
}

// Summary feeds the overview screen, /metrics and core.risk_daily.
type Summary struct {
	Customers struct {
		Total, High, Medium, Low, NoScore int
	}
	Services struct {
		Total, Live, AtRisk, ProlongWait int
	}
	Money struct {
		ExpectedLoss, WorstCase, MRRAtRisk, TotalMRR, SavableTop20 int64
	}
	TopAtRisk []int64 // up to 10 account ids by expected loss desc
}
