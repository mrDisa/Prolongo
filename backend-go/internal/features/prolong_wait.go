package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/textfmt"
)

func init() { Register(prolongWait{}) }

// prolongWait: the paid period is over and the service waits for payment;
// after 5 days of grace it is deleted.
type prolongWait struct{}

const graceDays = domain.GraceDays

func (prolongWait) Meta() Meta {
	return Meta{
		Key: "prolong_wait", Title: "Ожидает оплаты",
		Description: "Услуга в статусе prolong_wait: срок истёк, через 5 дней будет удалена.",
		Scope:       ScopeService, DefaultWeight: 45, Min: 0, Max: 100,
		Action: "Срочно: письмо или звонок со ссылкой на оплату, предложить отсрочку",
	}
}

// Compute: graceLeft = 5 + daysLeft; value = clamp(0.6 + 0.4 × (1 − graceLeft / 5), 0.6, 1).
// Fresh prolong_wait starts at 0.6 and grows to 1 as deletion approaches.
func (prolongWait) Compute(c *Context, s *domain.Service) (float64, string) {
	if s.Status != domain.StatusProlongWait {
		return 0, ""
	}
	grace := graceDays + DaysLeft(*s, c.Now)
	value := Clamp(0.6+0.4*(1-grace/graceDays), 0.6, 1)
	if grace > 0 {
		return value, fmt.Sprintf("%s ждёт оплаты, до удаления %s", s.Name, textfmt.Days(grace))
	}
	return value, fmt.Sprintf("%s: срок ожидания оплаты истёк %s назад — удаление в любой момент", s.Name, textfmt.Days(grace))
}
