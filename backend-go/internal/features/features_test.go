package features

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

var now = time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC)

// svc builds a service that expires `days` from now (negative = already expired).
func svc(status domain.ServiceStatus, term domain.PaymentTerm, days float64, auto bool) domain.Service {
	return domain.Service{
		ID: 1, Name: "service-0001", OwnerID: 1, PaymentTerm: term, AutoProlong: auto, Status: status, Price: 1000,
		CreatedAt: now.AddDate(-1, 0, 0), ExpiresAt: now.Add(HorizonDuration(days)),
	}
}

func compute(t *testing.T, key string, c *Context, s *domain.Service) (float64, string) {
	t.Helper()
	f, ok := ByKey(key)
	require.True(t, ok, key)
	return f.Compute(c, s)
}

type serviceCase struct {
	name   string
	s      domain.Service
	value  float64
	reason string
}

func runServiceCases(t *testing.T, key string, cases []serviceCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, r := compute(t, key, &Context{Now: now, HorizonDays: 30}, &tc.s)
			assert.InDelta(t, tc.value, v, 1e-9)
			assert.Equal(t, tc.reason, r)
		})
	}
}

func TestRegistry(t *testing.T) {
	all := All()
	keys := make([]string, len(all))
	for i, f := range all {
		keys[i] = f.Meta().Key
	}
	assert.Equal(t, []string{"churn_history", "expiring_no_autoprolong", "failed_payments", "insufficient_funds",
		"loyal_payer", "new_client", "prolong_wait", "short_billing", "suspended"}, keys, "9 features, sorted by key")

	for _, f := range all {
		m := f.Meta()
		assert.NotEmpty(t, m.Title, m.Key)
		assert.NotEmpty(t, m.Description, m.Key)
		assert.Contains(t, []Scope{ScopeService, ScopeAccount}, m.Scope, m.Key)
		assert.True(t, m.Min <= m.DefaultWeight && m.DefaultWeight <= m.Max, "%s: default weight within [min, max]", m.Key)
	}
	assert.Panics(t, func() { Register(suspended{}) }, "duplicate key")
}

func TestProlongWait(t *testing.T) {
	runServiceCases(t, "prolong_wait", []serviceCase{
		{"not waiting", svc(domain.StatusActive, domain.TermMonth, -1, false), 0, ""},
		{"just expired: 0.6", svc(domain.StatusProlongWait, domain.TermMonth, 0, false), 0.6, "service-0001 ждёт оплаты, до удаления 5 дн."},
		{"halfway: 0.8", svc(domain.StatusProlongWait, domain.TermMonth, -2.5, false), 0.8, "service-0001 ждёт оплаты, до удаления 3 дн."},
		{"grace over: 1", svc(domain.StatusProlongWait, domain.TermHour, -399.94, true), 1,
			"service-0001: срок ожидания оплаты истёк 395 дн. назад — удаление в любой момент"},
		{"paid ahead still clamps to 0.6", svc(domain.StatusProlongWait, domain.TermMonth, 3, false), 0.6, "service-0001 ждёт оплаты, до удаления 8 дн."},
	})
}

func TestExpiringNoAutoprolong(t *testing.T) {
	runServiceCases(t, "expiring_no_autoprolong", []serviceCase{
		{"auto-renewal on", svc(domain.StatusActive, domain.TermMonth, 1, true), 0, ""},
		{"prolong_wait is another feature", svc(domain.StatusProlongWait, domain.TermMonth, -1, false), 0, ""},
		{"expired: 1", svc(domain.StatusSuspended, domain.TermMonth, -39.98, false), 1, "service-0001 истекла 40 дн. назад, автопродление выключено"},
		{"month, 7 of 14 days left: 0.5", svc(domain.StatusActive, domain.TermMonth, 7, false), 0.5, "service-0001 истекает через 7 дн., автопродление выключено"},
		{"month, beyond 14-day window", svc(domain.StatusActive, domain.TermMonth, 20, false), 0, ""},
		{"week window is 7 days", svc(domain.StatusActive, domain.TermWeek, 3.5, false), 0.5, "service-0001 истекает через 4 дн., автопродление выключено"},
		{"hour window is 1 day", svc(domain.StatusActive, domain.TermHour, 0.0368, false), 1 - 0.0368, "service-0001 истекает через 1 ч, автопродление выключено"},
	})
}

func TestSuspended(t *testing.T) {
	runServiceCases(t, "suspended", []serviceCase{
		{"active", svc(domain.StatusActive, domain.TermMonth, 5, false), 0, ""},
		{"suspended", svc(domain.StatusSuspended, domain.TermMonth, 5, false), 1, "service-0001 выключен клиентом"},
	})
}

func TestShortBilling(t *testing.T) {
	runServiceCases(t, "short_billing", []serviceCase{
		{"hour", svc(domain.StatusActive, domain.TermHour, 1, false), 1, "Почасовая оплата — легко уйти"},
		{"week", svc(domain.StatusActive, domain.TermWeek, 1, false), 0.5, "Понедельная оплата — легко уйти"},
		{"month", svc(domain.StatusActive, domain.TermMonth, 1, false), 0, ""},
		{"year", svc(domain.StatusActive, domain.TermYear, 1, false), 0, ""},
	})
}

func TestInsufficientFunds(t *testing.T) {
	cases := []struct {
		name   string
		c      Context
		value  float64
		reason string
	}{
		{"nothing to pay", Context{NeedH: 0, Funds: 0}, 0, ""},
		{"enough money", Context{NeedH: 1000, Funds: 1500}, 0, ""},
		{"no money: 1", Context{NeedH: 1200, Funds: 0}, 1, "На балансе €0,00, на продления за 30 дн. нужно €12,00"},
		{"customer 5", Context{NeedH: 469765, Funds: 28657}, 1 - 28657.0/469765, "На балансе €286,57, на продления за 30 дн. нужно €4 697,65"},
		{"bonuses counted", Context{NeedH: 1000, Funds: 400, Account: domain.Account{BonusUnlocked: true, BonusBalance: 100}}, 0.6,
			"На балансе с бонусами €4,00, на продления за 30 дн. нужно €10,00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.c.HorizonDays = 30
			v, r := compute(t, "insufficient_funds", &tc.c, nil)
			assert.InDelta(t, tc.value, v, 1e-9)
			assert.Equal(t, tc.reason, r)
		})
	}
}

func tx(performed bool, kind domain.TxType) domain.Transaction {
	return domain.Transaction{Performed: performed, Type: kind}
}

func TestFailedPayments(t *testing.T) {
	v, r := compute(t, "failed_payments", &Context{}, nil)
	assert.Zero(t, v, "no transactions")
	assert.Empty(t, r)

	c := &Context{Tx: []domain.Transaction{tx(true, domain.TxOrder), tx(true, domain.TxProlong)}}
	v, _ = compute(t, "failed_payments", c, nil)
	assert.Zero(t, v, "all paid")

	c.Tx = append(c.Tx, tx(false, domain.TxProlong), tx(false, domain.TxProlong))
	v, r = compute(t, "failed_payments", c, nil)
	assert.InDelta(t, 0.5, v, 1e-9)
	assert.Equal(t, "2 из 4 платежей не прошли", r)
}

func TestChurnHistory(t *testing.T) {
	c := &Context{Services: []domain.Service{{Status: domain.StatusActive}, {Status: domain.StatusBlocked}}}
	v, _ := compute(t, "churn_history", c, nil)
	assert.Zero(t, v, "blocked is not churn")

	c.Services = append(c.Services, domain.Service{Status: domain.StatusDeleted}, domain.Service{Status: domain.StatusDeleted})
	v, r := compute(t, "churn_history", c, nil)
	assert.InDelta(t, 0.5, v, 1e-9)
	assert.Equal(t, "Уже удалил 2 из 4 услуг", r)
}

func TestNewClient(t *testing.T) {
	cases := []struct {
		name          string
		mrr, topUp    int64
		value         float64
		reasonPresent bool
	}{
		{"no live services", 0, 0, 0, false},
		{"topped up 3 months", 1000, 3000, 0, false},
		{"topped up more", 1000, 9000, 0, false},
		{"topped up 1 month", 1000, 1000, 1 - 1.0/3, true},
		{"never topped up", 1000, 0, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, r := compute(t, "new_client", &Context{MRR: tc.mrr, Account: domain.Account{TotalReplenished: tc.topUp}}, nil)
			assert.InDelta(t, tc.value, v, 1e-9)
			assert.Equal(t, tc.reasonPresent, r != "")
		})
	}
}

func TestLoyalPayer(t *testing.T) {
	c := &Context{Tx: []domain.Transaction{tx(true, domain.TxOrder), tx(false, domain.TxProlong)}}
	v, _ := compute(t, "loyal_payer", c, nil)
	assert.Zero(t, v, "orders and failed renewals do not count")

	c.Tx = append(c.Tx, tx(true, domain.TxProlong), tx(true, domain.TxProlong))
	v, r := compute(t, "loyal_payer", c, nil)
	assert.InDelta(t, 0.4, v, 1e-9)
	assert.Equal(t, "Стабильно продлевает: успешных продлений — 2", r)

	for range 6 {
		c.Tx = append(c.Tx, tx(true, domain.TxProlong))
	}
	v, _ = compute(t, "loyal_payer", c, nil)
	assert.InDelta(t, 1, v, 1e-9, "capped at 5 renewals")

	m := loyalPayer{}.Meta()
	assert.Negative(t, m.DefaultWeight)
	assert.Empty(t, m.Action, "nothing to recommend for a good sign")
}

func TestChargesH(t *testing.T) {
	cases := []struct {
		name string
		s    domain.Service
		h    float64
		want int64
	}{
		{"paid beyond the horizon", svc(domain.StatusActive, domain.TermYear, 200, false), 30, 0},
		{"expires inside: one month", svc(domain.StatusActive, domain.TermMonth, 10, false), 30, 1},
		{"expired hourly: every hour of 30 days", svc(domain.StatusProlongWait, domain.TermHour, -399.94, true), 30, 720},
		{"expired monthly: exactly one period", svc(domain.StatusProlongWait, domain.TermMonth, -370, true), 30, 1},
		{"half year: at least one", svc(domain.StatusProlongWait, domain.TermHalfYear, -219, false), 30, 1},
		{"weekly over 30 days", svc(domain.StatusActive, domain.TermWeek, 0, false), 30, 5},
		{"weekly over 7 days", svc(domain.StatusActive, domain.TermWeek, 0, false), 7, 1},
		{"expires exactly at the horizon end", svc(domain.StatusActive, domain.TermMonth, 30, false), 30, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ChargesH(tc.s, now, tc.h))
		})
	}
}

func TestMRR(t *testing.T) {
	s := svc(domain.StatusActive, domain.TermHour, 0, false)
	s.Price = 651
	assert.EqualValues(t, 468720, MRR(s), "651 × 24 × 30")
	s.PaymentTerm, s.Price = domain.TermHalfYear, 748
	assert.EqualValues(t, 123, MRR(s), "748 × 30 / 182 = 123.3")
}

func TestNewContext(t *testing.T) {
	acc := domain.Account{ID: 1, Balance: 500, BonusBalance: 200, BonusUnlocked: true}
	live := svc(domain.StatusActive, domain.TermMonth, 10, false)
	gone := svc(domain.StatusDeleted, domain.TermMonth, -100, false)
	gone.ID = 2
	txs := map[int64][]domain.Transaction{1: {tx(true, domain.TxOrder)}, 2: {tx(false, domain.TxProlong)}}

	c := NewContext(acc, []domain.Service{live, gone}, txs, now, 30)
	assert.Len(t, c.Services, 2)
	assert.Len(t, c.Live, 1)
	assert.Len(t, c.Tx, 2, "transactions of all services, deleted included")
	assert.EqualValues(t, 1000, c.NeedH)
	assert.EqualValues(t, 700, c.Funds, "unlocked bonus counts as funds")
	assert.EqualValues(t, 1000, c.MRR)
}
