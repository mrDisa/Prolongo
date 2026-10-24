package features

import (
	"math"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

func init() { Register(newClient{}) }

// newClient: the customer has topped up less than three months of their services.
type newClient struct{}

func (newClient) Meta() Meta {
	return Meta{
		Key: "new_client", Title: "Новый клиент",
		Description: "Пополнил меньше чем на 3 месяца своих услуг.",
		Scope:       ScopeAccount, DefaultWeight: 7, Min: 0, Max: 30,
		Action: "Онбординг: письмо с инструкциями, проверка настройки",
	}
}

// Compute: MRR > 0 → 1 − min(1, totalReplenished / (3 × MRR)), otherwise 0.
func (newClient) Compute(c *Context, _ *domain.Service) (float64, string) {
	if c.MRR <= 0 {
		return 0, ""
	}
	value := 1 - math.Min(1, float64(c.Account.TotalReplenished)/(3*float64(c.MRR)))
	if value == 0 {
		return 0, ""
	}
	return value, "Новый клиент: пополнил меньше чем на 3 месяца"
}
