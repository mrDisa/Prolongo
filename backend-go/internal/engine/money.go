package engine

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mrDisa/Prolongo/backend-go/internal/features"
	"github.com/mrDisa/Prolongo/backend-go/internal/textfmt"
)

// money estimates what the account brings and what we may lose within the horizon
// (docs/ОБЩЕЕ_ТЗ.md §5, docs/ТЗ_BACKEND_GO.md §9.3):
//
//	revenue  = Σ price × chargesH over live services
//	bonus    = bonusBalance if bonuses are unlocked, else 0
//	covered  = min(balance, Σ price × chargesH over live auto-renewing services)
//	atRisk   = max(0, revenue − bonus − covered)
//	expected = round(probability × atRisk)
//
// Bonus and balance coverage are capped so that the parts add up to revenue in the card.
func money(c *features.Context, probability float64, horizonDays int) Money {
	var autoNeed int64
	for _, s := range c.Live {
		if s.AutoProlong {
			autoNeed += s.Price * features.ChargesH(s, c.Now, c.HorizonDays)
		}
	}
	var bonus int64
	if c.Account.BonusUnlocked {
		bonus = c.Account.BonusBalance
	}

	m := Money{HorizonDays: horizonDays, Revenue: c.NeedH, MRR: c.MRR}
	m.BonusCovered = min(bonus, m.Revenue)
	m.BalanceCovered = min(c.Account.Balance, autoNeed, m.Revenue-m.BonusCovered)
	m.AtRisk = m.Revenue - m.BonusCovered - m.BalanceCovered
	m.ExpectedLoss = int64(math.Round(probability * float64(m.AtRisk)))
	m.Explanation = explain(m, probability, c.Account.BonusUnlocked, c.Account.BonusBalance)
	return m
}

// explain renders the calculation in words, e.g.:
// «Услуги принесут €10,00 за 30 дней. €2,00 покроют бонусы, €3,00 спишется с баланса автоматически.
// Под риском €5,00, вероятность ухода 72% → ожидаемые потери €3,60.»
func explain(m Money, probability float64, bonusUnlocked bool, bonusBalance int64) string {
	days := fmt.Sprintf("%d %s", m.HorizonDays, textfmt.Plural(m.HorizonDays, "день", "дня", "дней"))
	if m.Revenue == 0 {
		return fmt.Sprintf("За %s платежей по услугам не ожидается — потерь нет.", days)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Услуги принесут %s за %s.", textfmt.EUR(m.Revenue), days)

	var parts []string
	switch {
	case m.BonusCovered > 0:
		parts = append(parts, textfmt.EUR(m.BonusCovered)+" покроют бонусы")
	case !bonusUnlocked && bonusBalance > 0:
		parts = append(parts, "бонусы заблокированы")
	}
	if m.BalanceCovered > 0 {
		parts = append(parts, textfmt.EUR(m.BalanceCovered)+" спишется с баланса автоматически")
	}
	if len(parts) > 0 {
		b.WriteString(" " + capitalize(strings.Join(parts, ", ")) + ".")
	}

	if m.AtRisk == 0 {
		fmt.Fprintf(&b, " Все платежи за %s покрыты бонусами и автосписанием с баланса — потерь не ожидается.", days)
		return b.String()
	}
	fmt.Fprintf(&b, " Под риском %s, вероятность ухода %.0f%% → ожидаемые потери %s.",
		textfmt.EUR(m.AtRisk), math.Round(probability*100), textfmt.EUR(m.ExpectedLoss))
	return b.String()
}

// capitalize upper-cases the first letter. Strings are UTF-8 bytes: "б" is 2 bytes and "€" is 3,
// so s[:1] would cut a letter in half — decode the first rune instead.
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
