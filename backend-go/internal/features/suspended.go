package features

import (
	"fmt"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

func init() { Register(suspended{}) }

// suspended: the customer turned the server off — often the first step to leaving.
type suspended struct{}

func (suspended) Meta() Meta {
	return Meta{
		Key: "suspended", Title: "Сервер выключен",
		Description: "Клиент сам выключил сервер — часто это первый шаг к отказу.",
		Scope:       ScopeService, DefaultWeight: 8, Min: 0, Max: 50,
		Action: "Узнать, нужен ли сервер; предложить тариф дешевле",
	}
}

func (suspended) Compute(_ *Context, s *domain.Service) (float64, string) {
	if s.Status != domain.StatusSuspended {
		return 0, ""
	}
	return 1, fmt.Sprintf("%s выключен клиентом", s.Name)
}
