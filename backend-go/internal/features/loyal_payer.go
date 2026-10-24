package features

import (
	"fmt"
	"math"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

func init() { Register(loyalPayer{}) }

// loyalPayer lowers the risk: successful renewals show the customer keeps paying.
// Its weight is negative, so it subtracts points.
type loyalPayer struct{}

const loyalRenewals = 5 // renewals for the full effect

func (loyalPayer) Meta() Meta {
	return Meta{
		Key: "loyal_payer", Title: "Стабильно продлевает",
		Description: "Успешные продления снижают риск — до 5 продлений.",
		Scope:       ScopeAccount, DefaultWeight: -10, Min: -50, Max: 0,
		// No action: the feature lowers the risk, there is nothing to fix.
	}
}

// Compute: min(1, performed renewals / 5).
func (loyalPayer) Compute(c *Context, _ *domain.Service) (float64, string) {
	renewals := 0
	for _, t := range c.Tx {
		if t.Type == domain.TxProlong && t.Performed {
			renewals++
		}
	}
	if renewals == 0 {
		return 0, ""
	}
	return math.Min(1, float64(renewals)/loyalRenewals), fmt.Sprintf("Стабильно продлевает: успешных продлений — %d", renewals)
}
