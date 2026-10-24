package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/textfmt"
)

func init() { Register(expiringNoAutoprolong{}) }

// expiringNoAutoprolong: the paid period ends soon (or has ended) and nothing will renew it.
type expiringNoAutoprolong struct{}

func (expiringNoAutoprolong) Meta() Meta {
	return Meta{
		Key: "expiring_no_autoprolong", Title: "Истекает без автопродления",
		Description: "Срок оплаты скоро закончится или уже прошёл, а автопродление выключено.",
		Scope:       ScopeService, DefaultWeight: 30, Min: 0, Max: 100,
		Action: "Напомнить о продлении, предложить включить автопродление (бонус за включение)",
	}
}

// Compute for active / suspended services without auto-renewal:
// window = clamp(period, 1, 14) days; already expired → 1; within the window → 1 − daysLeft / window.
func (expiringNoAutoprolong) Compute(c *Context, s *domain.Service) (float64, string) {
	if s.AutoProlong || (s.Status != domain.StatusActive && s.Status != domain.StatusSuspended) {
		return 0, ""
	}
	left := DaysLeft(*s, c.Now)
	if left < 0 {
		return 1, fmt.Sprintf("%s истекла %s назад, автопродление выключено", s.Name, textfmt.Days(left))
	}
	window := Clamp(domain.TermDays(s.PaymentTerm), 1, 14)
	if left > window {
		return 0, ""
	}
	return 1 - left/window, fmt.Sprintf("%s истекает через %s, автопродление выключено", s.Name, textfmt.Days(left))
}
