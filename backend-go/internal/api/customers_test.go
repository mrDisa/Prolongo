package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getJSON[T any](t *testing.T, h http.Handler, cookie *http.Cookie, path string) (int, T) {
	t.Helper()
	rec := do(h, http.MethodGet, path, "", cookie)
	var v T
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	}
	return rec.Code, v
}

func ids(items []customerListItemJSON) []int64 {
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestListDefaults(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	code, list := getJSON[customerListJSON](t, h, login(t, h), "/api/v1/customers")
	require.Equal(t, http.StatusOK, code)

	assert.Equal(t, []int64{9, 5, 1, 4, 6, 10, 2, 3, 7, 8}, ids(list.Items),
		"expected loss desc, ties by id, customers without a score last")
	assert.Equal(t, 10, list.Total)
	assert.Equal(t, 1, list.Page)
	assert.Equal(t, 25, list.PageSize)
	assert.EqualValues(t, 17, list.CalcVersion)
	assert.Equal(t, "2026-10-19T14:35:54.204Z", list.CalculatedAt.Format("2006-01-02T15:04:05.000Z"))
}

// The list item of customer 5 must look like the openapi.yml example the frontend was built on.
func TestListItemMatchesContract(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodGet, "/api/v1/customers?search=user-0005", "", login(t, h))
	require.Equal(t, http.StatusOK, rec.Code)
	var raw struct{ Items []json.RawMessage }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Len(t, raw.Items, 1)
	assert.JSONEq(t, `{
		"id": 5, "email": "user-0005@example.test", "balanceCents": 28657, "totalReplenishedCents": 293608,
		"scoreStatus": "scored", "noScoreReason": null, "riskScore": 77, "riskLevel": "high", "loyalty": 23,
		"probability": 0.7671, "expectedLossCents": 338372, "mrrCents": 469140, "liveServices": 3,
		"topReasons": [
			{"feature": "prolong_wait", "points": 45, "text": "service-0025: срок ожидания оплаты истёк 395 дн. назад — удаление в любой момент"},
			{"feature": "insufficient_funds", "points": 28.2, "text": "На балансе €286,57, на продления за 30 дн. нужно €4 697,65"},
			{"feature": "new_client", "points": 5.5, "text": "Новый клиент: пополнил меньше чем на 3 месяца"}
		]}`, string(raw.Items[0]))
}

func TestListNoScoreItemHasNulls(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodGet, "/api/v1/customers?search=user-0002", "", login(t, h))
	var raw struct{ Items []json.RawMessage }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.Len(t, raw.Items, 1)
	assert.JSONEq(t, `{
		"id": 2, "email": "user-0002@example.test", "balanceCents": 32098, "totalReplenishedCents": 105256,
		"scoreStatus": "no_score", "noScoreReason": "blocked", "riskScore": null, "riskLevel": null,
		"loyalty": null, "probability": null, "expectedLossCents": 0, "mrrCents": 0, "liveServices": 0,
		"topReasons": []}`, string(raw.Items[0]))
}

// docs/ТЗ_BACKEND_GO.md §16: every field × both directions, stable, no-score last for risk and money.
func TestSortEveryFieldAndDirection(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	snap := referenceSnapshot(t).snap

	key := func(field string, id int64) float64 {
		cr, _ := snap.Customer(id)
		switch field {
		case "id":
			return float64(id)
		case "balance":
			return float64(cr.Account.Balance)
		case "totalReplenished":
			return float64(cr.Account.TotalReplenished)
		case "riskScore":
			return cr.RiskScoreRaw
		case "expectedLoss":
			return float64(cr.Money.ExpectedLoss)
		}
		return 0
	}
	scored := func(id int64) bool { cr, _ := snap.Customer(id); return cr.ScoreStatus == "scored" }

	for _, field := range sortFields {
		for _, order := range []string{"asc", "desc"} {
			t.Run(field+"_"+order, func(t *testing.T) {
				_, list := getJSON[customerListJSON](t, h, cookie, fmt.Sprintf("/api/v1/customers?sort=%s&order=%s", field, order))
				got := ids(list.Items)
				require.Len(t, got, 10)
				for i := 1; i < len(got); i++ {
					a, b := got[i-1], got[i]
					if field == "riskScore" || field == "expectedLoss" {
						assert.False(t, !scored(a) && scored(b), "no-score %d before scored %d", a, b)
						if scored(a) != scored(b) {
							continue
						}
					}
					var c int
					if field == "email" {
						ea, _ := snap.Customer(a)
						eb, _ := snap.Customer(b)
						c = strings.Compare(strings.ToLower(ea.Account.Email), strings.ToLower(eb.Account.Email))
					} else {
						switch ka, kb := key(field, a), key(field, b); {
						case ka < kb:
							c = -1
						case ka > kb:
							c = 1
						}
					}
					if order == "desc" {
						c = -c
					}
					assert.True(t, c < 0 || (c == 0 && a < b), "%d before %d breaks %s %s", a, b, field, order)
				}
			})
		}
	}

	_, asc := getJSON[customerListJSON](t, h, cookie, "/api/v1/customers?sort=riskScore&order=asc")
	assert.Equal(t, []int64{6, 4, 1, 10, 9, 5, 2, 3, 7, 8}, ids(asc.Items), "no-score last in asc too")
}

func TestListFiltersAndSearch(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	cases := map[string][]int64{
		"level=high,medium":   {9, 5, 1, 10},
		"level=none":          {2, 3, 7, 8},
		"level=low":           {4, 6},
		"search=user-000":     {9, 5, 1, 4, 6, 2, 3, 7, 8},
		"search=USER-0003":    {3},
		"search=5":            {5},
		"search=10":           {10},
		"search=nobody":       {},
		"level=high&search=9": {9},
	}
	for query, want := range cases {
		_, list := getJSON[customerListJSON](t, h, cookie, "/api/v1/customers?"+query)
		assert.Equal(t, want, ids(list.Items), query)
		assert.Equal(t, len(want), list.Total, "%s: total after the filter", query)
	}
}

func TestListPagination(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	_, p2 := getJSON[customerListJSON](t, h, cookie, "/api/v1/customers?pageSize=3&page=2")
	assert.Equal(t, []int64{4, 6, 10}, ids(p2.Items))
	assert.Equal(t, 10, p2.Total)
	assert.Equal(t, 2, p2.Page)
	assert.Equal(t, 3, p2.PageSize)

	_, last := getJSON[customerListJSON](t, h, cookie, "/api/v1/customers?pageSize=3&page=4")
	assert.Equal(t, []int64{8}, ids(last.Items))

	rec := do(h, http.MethodGet, "/api/v1/customers?pageSize=3&page=9", "", cookie)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"items":[]`, "past the end: an empty page, not null")
}

func TestListBadParams(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	cases := map[string]string{
		"sort=score":     "sort: допустимые значения id, email, balance, totalReplenished, riskScore, expectedLoss",
		"order=up":       "order: допустимые значения asc, desc",
		"level=critical": "level: допустимые значения high, medium, low, none через запятую",
		"page=0":         "page: целое число от 1",
		"pageSize=500":   "pageSize: целое число от 1 до 200",
		"pageSize=abc":   "pageSize: целое число от 1 до 200",
	}
	for query, msg := range cases {
		rec := do(h, http.MethodGet, "/api/v1/customers?"+query, "", cookie)
		assert.Equal(t, http.StatusBadRequest, rec.Code, query)
		assert.JSONEq(t, `{"error":{"code":"bad_request","message":"`+msg+`"}}`, rec.Body.String(), query)
	}
}

func TestCustomerDetail(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	code, d := getJSON[customerDetailJSON](t, h, login(t, h), "/api/v1/customers/5")
	require.Equal(t, http.StatusOK, code)

	assert.Equal(t, "locked", d.BonusState)
	assert.EqualValues(t, 7623, d.WithdrawBalanceCents)
	assert.Equal(t, 77, *d.RiskScore)
	assert.EqualValues(t, 25, *d.RiskiestServiceID)
	assert.Equal(t, moneyJSON{HorizonDays: 30, RevenueCents: 469765, BonusCoveredCents: 0, BalanceCoveredCents: 28657,
		AtRiskCents: 441108, ExpectedLossCents: 338372, MRRCents: 469140,
		Explanation: "Услуги принесут €4 697,65 за 30 дней. Бонусы заблокированы, €286,57 спишется с баланса автоматически. " +
			"Под риском €4 411,08, вероятность ухода 77% → ожидаемые потери €3 383,72."}, d.Money)

	require.Len(t, d.Contributions, 6)
	assert.Equal(t, contributionJSON{Feature: "insufficient_funds", Title: "Не хватает денег на продление", Scope: "account",
		Value: 0.939, Weight: 30, Points: 28.2, Text: "На балансе €286,57, на продления за 30 дн. нужно €4 697,65"}, d.Contributions[1])
	assert.EqualValues(t, 25, *d.Contributions[0].ServiceID, "service feature points to the service")
	assert.Nil(t, d.Contributions[1].ServiceID, "account feature: null")
	assert.Len(t, d.Recommendations, 4)

	var services []int64
	for _, s := range d.Services {
		services = append(services, s.ID)
	}
	assert.Equal(t, []int64{25, 5, 15}, services, "riskiest first")
	assert.Equal(t, -399.9, d.Services[0].DaysLeft)
	assert.EqualValues(t, 468720, d.Services[0].MRRCents)

	assert.Len(t, d.Transactions, 10)
	assert.True(t, slices.IsSortedFunc(d.Transactions, func(a, b transactionItemJSON) int { return b.CreatedAt.Compare(a.CreatedAt) }),
		"newest payments first")
	require.Len(t, d.StatusHistory, 1)
	assert.Equal(t, "prolong_wait", d.StatusHistory[0].ToStatus)
	assert.EqualValues(t, 17, d.CalcVersion)
}

func TestCustomerDetailNoScore(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodGet, "/api/v1/customers/3", "", login(t, h))
	require.Equal(t, http.StatusOK, rec.Code)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	for field, want := range map[string]string{
		"scoreStatus": `"no_score"`, "noScoreReason": `"churned"`, "riskScore": "null", "riskLevel": "null",
		"loyalty": "null", "probability": "null", "riskiestServiceId": "null", "contributions": "[]",
		"recommendations": "[]", "statusHistory": "[]",
	} {
		assert.JSONEq(t, want, string(raw[field]), field)
	}
	var d customerDetailJSON
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Zero(t, d.Money.ExpectedLossCents)
	assert.Len(t, d.Services, 3, "deleted services are still listed")
	for _, s := range d.Services {
		assert.Nil(t, s.RiskScore, "not live: no score")
	}
}

func TestCustomerDetailErrors(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	rec := do(h, http.MethodGet, "/api/v1/customers/999", "", cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"not_found","message":"Клиент не найден"}}`, rec.Body.String())

	for _, bad := range []string{"abc", "0", "-5"} {
		rec = do(h, http.MethodGet, "/api/v1/customers/"+bad, "", cookie)
		assert.Equal(t, http.StatusBadRequest, rec.Code, bad)
	}
}

func TestSummary(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	code, s := getJSON[summaryJSON](t, h, login(t, h), "/api/v1/analytics/summary")
	require.Equal(t, http.StatusOK, code)

	assert.Equal(t, "2026-10-19T14:35:53Z", s.ReferenceTime.Format("2006-01-02T15:04:05Z07:00"))
	assert.Equal(t, "data", s.ClockMode)
	assert.Equal(t, 30, s.HorizonDays)
	assert.Equal(t, [5]int{10, 2, 2, 2, 4}, [5]int{s.Customers.Total, s.Customers.High, s.Customers.Medium, s.Customers.Low, s.Customers.NoScore})
	assert.Equal(t, [4]int{30, 18, 10, 6}, [4]int{s.Services.Total, s.Services.Live, s.Services.AtRisk, s.Services.ProlongWait})
	assert.Equal(t, [5]int64{703267, 966052, 1155577, 1159275, 703267},
		[5]int64{s.Money.ExpectedLossCents, s.Money.WorstCaseLossCents, s.Money.MRRAtRiskCents, s.Money.TotalMRRCents, s.Money.SavableTop20Cents})
	assert.Equal(t, []int64{9, 5, 1, 4, 6, 10}, ids(s.TopAtRisk))
}
