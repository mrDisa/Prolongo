package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/textfmt"
)

// "What if…" on the customer card: retention actions applied to a copy of one customer's
// data, scored by the same engine. Nothing is saved and no SSE is sent, like the model preview.

const (
	actionTopUp             = "top_up"
	actionEnableAutoProlong = "enable_auto_prolong"
	actionPayWaiting        = "pay_waiting"

	maxWhatIfActions = 10
	maxTopUpCents    = 10_000_000 // €100 000
	maxWhatIfBody    = 16 << 10
)

type whatIfActionJSON struct {
	Type        string `json:"type"`
	AmountCents *int64 `json:"amountCents,omitempty"`
	ServiceID   *int64 `json:"serviceId,omitempty"`
}

type whatIfRequestJSON struct {
	Actions []whatIfActionJSON `json:"actions"`
}

type whatIfStateJSON struct {
	ScoreStatus   string       `json:"scoreStatus"`
	NoScoreReason *string      `json:"noScoreReason"`
	RiskScore     *int         `json:"riskScore"`
	RiskLevel     *string      `json:"riskLevel"`
	Probability   *float64     `json:"probability"`
	BalanceCents  int64        `json:"balanceCents"`
	Money         moneyJSON    `json:"money"`
	Contributions []reasonJSON `json:"contributions"`
}

type whatIfJSON struct {
	Before     whatIfStateJSON `json:"before"`
	After      whatIfStateJSON `json:"after"`
	SavedCents int64           `json:"savedCents"` // before − after expected loss; negative when the risk grows
	Notes      []string        `json:"notes"`      // what each action did, in words
}

// customerData is one customer's slice of the dataset, copied so that actions can change it.
type customerData struct {
	account  domain.Account
	services []domain.Service
	txs      map[int64][]domain.Transaction
}

func copyCustomer(ds *domain.Dataset, acc domain.Account) customerData {
	c := customerData{account: acc, services: slices.Clone(ds.ServicesByAccount[acc.ID]), txs: map[int64][]domain.Transaction{}}
	for _, s := range c.services {
		c.txs[s.ID] = slices.Clone(ds.TxByService[s.ID])
	}
	return c
}

func (c customerData) dataset(src *domain.Dataset) *domain.Dataset {
	return &domain.Dataset{
		Accounts:          []domain.Account{c.account},
		ServicesByAccount: map[int64][]domain.Service{c.account.ID: c.services},
		TxByService:       c.txs,
		LatestTimestamp:   src.LatestTimestamp,
		SourceTime:        src.SourceTime,
	}
}

// pay takes the price from unlocked bonuses first, then from the balance (as the billing does).
func (c *customerData) pay(price int64) bool {
	bonus := int64(0)
	if c.account.BonusUnlocked {
		bonus = c.account.BonusBalance
	}
	if bonus+c.account.Balance < price {
		return false
	}
	fromBonus := min(price, bonus)
	c.account.BonusBalance -= fromBonus
	c.account.Balance -= price - fromBonus
	c.account.WithdrawBalance = min(c.account.WithdrawBalance, c.account.Balance)
	return true
}

func (c *customerData) apply(a whatIfActionJSON, now time.Time) []string {
	switch a.Type {
	case actionTopUp:
		c.account.Balance += *a.AmountCents
		c.account.TotalReplenished += *a.AmountCents
		return []string{"Пополнит баланс на " + textfmt.EUR(*a.AmountCents)}
	case actionEnableAutoProlong:
		var notes []string
		for i := range c.services {
			s := &c.services[i]
			if s.Live() && !s.AutoProlong && (a.ServiceID == nil || *a.ServiceID == s.ID) {
				s.AutoProlong = true
				notes = append(notes, s.Name+": включит автопродление")
			}
		}
		if len(notes) == 0 {
			return []string{"Автопродление уже включено на всех живых услугах"}
		}
		return notes
	case actionPayWaiting:
		var notes []string
		for i := range c.services {
			s := &c.services[i]
			if s.Status != domain.StatusProlongWait {
				continue
			}
			if !c.pay(s.Price) {
				notes = append(notes, s.Name+": не хватит денег на оплату "+textfmt.EUR(s.Price))
				continue
			}
			s.Status = domain.StatusActive
			s.ExpiresAt = now.Add(domain.TermDuration(s.PaymentTerm))
			paidAt := now
			c.txs[s.ID] = append(c.txs[s.ID], domain.Transaction{
				ServiceID: s.ID, Amount: min(s.Price, 1000), Performed: true, PerformedAt: &paidAt,
				CreatedAt: now, Type: domain.TxProlong,
			})
			notes = append(notes, s.Name+": оплатит "+textfmt.EUR(s.Price)+", услуга снова активна")
		}
		if len(notes) == 0 {
			return []string{"Нет услуг, ожидающих оплаты"}
		}
		return notes
	}
	return nil
}

// validate returns error details keyed by path, as in every 422 of the API.
func (req whatIfRequestJSON) validate(services []domain.Service) map[string][]string {
	errs := map[string][]string{}
	if len(req.Actions) == 0 || len(req.Actions) > maxWhatIfActions {
		errs["actions"] = []string{fmt.Sprintf("От 1 до %d действий", maxWhatIfActions)}
	}
	for i, a := range req.Actions {
		path := fmt.Sprintf("actions[%d]", i)
		switch a.Type {
		case actionTopUp:
			if a.AmountCents == nil || *a.AmountCents < 1 || *a.AmountCents > maxTopUpCents {
				errs[path+".amountCents"] = []string{"Сумма в центах от 1 до " + strconv.Itoa(maxTopUpCents)}
			}
		case actionEnableAutoProlong:
			if a.ServiceID != nil && !slices.ContainsFunc(services, func(s domain.Service) bool { return s.ID == *a.ServiceID }) {
				errs[path+".serviceId"] = []string{"У клиента нет такой услуги"}
			}
		case actionPayWaiting:
		default:
			errs[path+".type"] = []string{"Допустимые значения: top_up, enable_auto_prolong, pay_waiting"}
		}
		if a.Type != actionTopUp && a.AmountCents != nil {
			errs[path+".amountCents"] = append(errs[path+".amountCents"], "Только для top_up")
		}
		if a.Type != actionEnableAutoProlong && a.ServiceID != nil {
			errs[path+".serviceId"] = append(errs[path+".serviceId"], "Только для enable_auto_prolong")
		}
	}
	return errs
}

func toWhatIfState(cr *engine.CustomerResult) whatIfStateJSON {
	st := whatIfStateJSON{
		ScoreStatus: cr.ScoreStatus, BalanceCents: cr.Account.Balance, Money: toMoneyJSON(cr.Money),
		Contributions: []reasonJSON{},
	}
	st.NoScoreReason, st.RiskScore, st.RiskLevel, _, st.Probability = scoreFields(cr)
	for _, c := range cr.Contributions {
		st.Contributions = append(st.Contributions, reasonJSON{Feature: c.Feature, Points: roundTo(c.Points, 1), Text: c.Text})
	}
	return st
}

// whatIf: POST /api/v1/customers/{id}/what-if.
func (h *handlers) whatIf(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "id: целое число от 1", nil)
		return
	}
	days, err := parseHorizon(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error(), nil)
		return
	}
	var req whatIfRequestJSON
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxWhatIfBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Ожидается JSON по схеме WhatIfRequest",
			map[string][]string{"body": {err.Error()}})
		return
	}
	snap, ok := h.snapshot(w, r)
	if !ok {
		return
	}
	idx := slices.IndexFunc(snap.Dataset.Accounts, func(a domain.Account) bool { return a.ID == id })
	if idx < 0 {
		writeError(w, http.StatusNotFound, codeNotFound, "Клиент не найден", nil)
		return
	}
	acc := snap.Dataset.Accounts[idx]
	if errs := req.validate(snap.Dataset.ServicesByAccount[id]); len(errs) > 0 {
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Действия не прошли проверку", errs)
		return
	}

	settings := snap.Settings
	if days != 0 {
		settings.HorizonDays = days
	}
	before := copyCustomer(snap.Dataset, acc)
	after := copyCustomer(snap.Dataset, acc)
	notes := []string{}
	for _, a := range req.Actions {
		notes = append(notes, after.apply(a, snap.Now)...)
	}

	b, _ := engine.Calculate(before.dataset(snap.Dataset), settings, snap.Now).Customer(id)
	a, _ := engine.Calculate(after.dataset(snap.Dataset), settings, snap.Now).Customer(id)
	writeJSON(w, http.StatusOK, whatIfJSON{
		Before: toWhatIfState(b), After: toWhatIfState(a),
		SavedCents: b.Money.ExpectedLoss - a.Money.ExpectedLoss, Notes: notes,
	})
}
