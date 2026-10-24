package features

import (
	"math"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

// Clamp limits x to [lo, hi].
func Clamp(x, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, x))
}

// DaysLeft is (expiresAt − now) in days; negative when the paid period is over.
func DaysLeft(s domain.Service, now time.Time) float64 {
	return s.ExpiresAt.Sub(now).Hours() / 24
}

// HorizonDuration converts a horizon in days to a duration.
func HorizonDuration(days float64) time.Duration {
	return time.Duration(days * float64(24*time.Hour))
}

// ChargesH counts how many times the service will be paid within the horizon if the customer stays
// (docs/ОБЩЕЕ_ТЗ.md §4.1):
//   - paid beyond now+H → 0;
//   - otherwise max(1, ceil((now+H − max(expiresAt, now)) / period)).
//
// An expired or prolong_wait service counts from now: if the customer pays, it runs the whole horizon.
func ChargesH(s domain.Service, now time.Time, horizonDays float64) int64 {
	end := now.Add(HorizonDuration(horizonDays))
	if s.ExpiresAt.After(end) {
		return 0
	}
	period := domain.TermDuration(s.PaymentTerm)
	if period <= 0 {
		return 0
	}
	start := s.ExpiresAt
	if start.Before(now) {
		start = now
	}
	span := end.Sub(start)
	n := int64((span + period - 1) / period) // ceil division on integer nanoseconds
	return max(1, n)
}

// MRR is the monthly revenue of a service in cents: price × 30 / period length in days.
func MRR(s domain.Service) int64 {
	days := domain.TermDays(s.PaymentTerm)
	if days <= 0 {
		return 0
	}
	return int64(math.Round(float64(s.Price) * 30 / days))
}

// NewContext builds the per-account context shared by all features.
func NewContext(acc domain.Account, services []domain.Service, txByService map[int64][]domain.Transaction, now time.Time, horizonDays float64) *Context {
	c := &Context{Now: now, HorizonDays: horizonDays, Account: acc, Services: services, Funds: acc.Balance}
	if acc.BonusUnlocked {
		c.Funds += acc.BonusBalance
	}
	for _, s := range services {
		c.Tx = append(c.Tx, txByService[s.ID]...)
		if !s.Live() {
			continue
		}
		c.Live = append(c.Live, s)
		c.NeedH += s.Price * ChargesH(s, now, horizonDays)
		c.MRR += MRR(s)
	}
	return c
}
