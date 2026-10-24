package features

import (
	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

func init() { Register(shortBilling{}) }

// shortBilling: hourly or weekly billing — nothing holds the customer, leaving is one click.
type shortBilling struct{}

func (shortBilling) Meta() Meta {
	return Meta{
		Key: "short_billing", Title: "Короткий период оплаты",
		Description: "Почасовая или понедельная оплата — уйти проще.",
		Scope:       ScopeService, DefaultWeight: 5, Min: 0, Max: 30,
		Action: "Предложить помесячную оплату со скидкой",
	}
}

// Compute: hour → 1, week → 0.5, longer periods → 0.
func (shortBilling) Compute(_ *Context, s *domain.Service) (float64, string) {
	switch s.PaymentTerm {
	case domain.TermHour:
		return 1, "Почасовая оплата — легко уйти"
	case domain.TermWeek:
		return 0.5, "Понедельная оплата — легко уйти"
	}
	return 0, ""
}
