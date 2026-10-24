package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/textfmt"
)

func init() { Register(insufficientFunds{}) }

// insufficientFunds: balance and usable bonuses do not cover the renewals due within the horizon.
type insufficientFunds struct{}

func (insufficientFunds) Meta() Meta {
	return Meta{
		Key: "insufficient_funds", Title: "Не хватает денег на продление",
		Description: "Баланса и доступных бонусов не хватит на продления за горизонт.",
		Scope:       ScopeAccount, DefaultWeight: 30, Min: 0, Max: 100,
		Action: "Предложить пополнение с бонусом +10%",
	}
}

// Compute: needH > 0 → clamp(1 − funds / needH, 0, 1), otherwise 0.
func (insufficientFunds) Compute(c *Context, _ *domain.Service) (float64, string) {
	if c.NeedH <= 0 {
		return 0, ""
	}
	value := Clamp(1-float64(c.Funds)/float64(c.NeedH), 0, 1)
	if value == 0 {
		return 0, ""
	}
	where := "На балансе"
	if c.Account.BonusUnlocked && c.Account.BonusBalance > 0 {
		where = "На балансе с бонусами"
	}
	return value, fmt.Sprintf("%s %s, на продления за %.0f дн. нужно %s",
		where, textfmt.EUR(c.Funds), c.HorizonDays, textfmt.EUR(c.NeedH))
}
