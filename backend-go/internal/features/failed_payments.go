package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

func init() { Register(failedPayments{}) }

// failedPayments: share of the account's payments that did not go through.
type failedPayments struct{}

func (failedPayments) Meta() Meta {
	return Meta{
		Key: "failed_payments", Title: "Неудачные платежи",
		Description: "Доля платежей клиента, которые не прошли.",
		Scope:       ScopeAccount, DefaultWeight: 10, Min: 0, Max: 50,
		Action: "Проверить способ оплаты, помочь с ошибкой платежа",
	}
}

// Compute: not_performed / all transactions; no transactions → 0.
func (failedPayments) Compute(c *Context, _ *domain.Service) (float64, string) {
	if len(c.Tx) == 0 {
		return 0, ""
	}
	failed := 0
	for _, t := range c.Tx {
		if !t.Performed {
			failed++
		}
	}
	if failed == 0 {
		return 0, ""
	}
	return float64(failed) / float64(len(c.Tx)), fmt.Sprintf("%d из %d платежей не прошли", failed, len(c.Tx))
}
