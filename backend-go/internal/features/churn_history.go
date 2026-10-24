package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

func init() { Register(churnHistory{}) }

// churnHistory: the customer has already deleted some services — they have left before.
type churnHistory struct{}

func (churnHistory) Meta() Meta {
	return Meta{
		Key: "churn_history", Title: "Уже удалял услуги",
		Description: "Доля удалённых услуг клиента — он уже уходил.",
		Scope:       ScopeAccount, DefaultWeight: 10, Min: 0, Max: 50,
		Action: "Опрос о причинах, персональная скидка",
	}
}

// Compute: deleted / all services of the account.
func (churnHistory) Compute(c *Context, _ *domain.Service) (float64, string) {
	if len(c.Services) == 0 {
		return 0, ""
	}
	deleted := 0
	for _, s := range c.Services {
		if s.Status == domain.StatusDeleted {
			deleted++
		}
	}
	if deleted == 0 {
		return 0, ""
	}
	return float64(deleted) / float64(len(c.Services)), fmt.Sprintf("Уже удалил %d из %d услуг", deleted, len(c.Services))
}
