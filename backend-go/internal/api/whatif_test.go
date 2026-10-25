package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postWhatIf(t *testing.T, h http.Handler, cookie *http.Cookie, path, body string) (int, whatIfJSON, string) {
	t.Helper()
	rec := do(h, http.MethodPost, path, body, cookie)
	var out whatIfJSON
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	}
	return rec.Code, out, rec.Body.String()
}

// "Before" is exactly the published card: the customer is scored alone on a copy of its data.
func TestWhatIfBeforeMatchesCard(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	code, out, body := postWhatIf(t, h, cookie, "/api/v1/customers/5/what-if", `{"actions":[{"type":"top_up","amountCents":0}]}`)
	require.Equal(t, http.StatusUnprocessableEntity, code, body)

	code, out, body = postWhatIf(t, h, cookie, "/api/v1/customers/5/what-if", `{"actions":[{"type":"enable_auto_prolong"}]}`)
	require.Equal(t, http.StatusOK, code, body)
	require.NotNil(t, out.Before.RiskScore)
	assert.Equal(t, 77, *out.Before.RiskScore)
	assert.EqualValues(t, 338372, out.Before.Money.ExpectedLossCents)
	assert.EqualValues(t, 28657, out.Before.BalanceCents)
	assert.Equal(t, "prolong_wait", out.Before.Contributions[0].Feature)
}

// Client 5 has three services waiting for payment: a top-up that covers them, then paying,
// removes the prolong_wait reason and most of the expected loss.
func TestWhatIfTopUpAndPay(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	code, out, body := postWhatIf(t, h, cookie, "/api/v1/customers/5/what-if",
		`{"actions":[{"type":"top_up","amountCents":500000},{"type":"pay_waiting"}]}`)
	require.Equal(t, http.StatusOK, code, body)

	require.NotNil(t, out.After.RiskScore)
	assert.Less(t, *out.After.RiskScore, *out.Before.RiskScore)
	assert.Less(t, out.After.Money.ExpectedLossCents, out.Before.Money.ExpectedLossCents)
	assert.Equal(t, out.Before.Money.ExpectedLossCents-out.After.Money.ExpectedLossCents, out.SavedCents)
	for _, c := range out.After.Contributions {
		assert.NotEqual(t, "prolong_wait", c.Feature, "every waiting service is paid")
	}
	assert.Len(t, out.Notes, 4, "the top-up and three paid services")
	assert.Contains(t, out.Notes[0], "Пополнит баланс на €5 000,00")
}

func TestWhatIfNotEnoughMoney(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	// Client 5 has €286.57 on the balance and locked bonuses: the hourly service (€6.51) is paid first.
	code, out, body := postWhatIf(t, h, cookie, "/api/v1/customers/5/what-if", `{"actions":[{"type":"pay_waiting"}]}`)
	require.Equal(t, http.StatusOK, code, body)
	assert.Len(t, out.Notes, 3)
	assert.Less(t, out.After.BalanceCents, out.Before.BalanceCents)
}

func TestWhatIfValidation(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	code, _, body := postWhatIf(t, h, cookie, "/api/v1/customers/5/what-if",
		`{"actions":[{"type":"refund"},{"type":"top_up"},{"type":"enable_auto_prolong","serviceId":1},{"type":"pay_waiting","amountCents":5}]}`)
	require.Equal(t, http.StatusUnprocessableEntity, code)
	var e errorBody
	require.NoError(t, json.Unmarshal([]byte(body), &e))
	assert.Equal(t, codeValidation, e.Error.Code)
	assert.Contains(t, e.Error.Details, "actions[0].type")
	assert.Contains(t, e.Error.Details, "actions[1].amountCents")
	assert.Contains(t, e.Error.Details, "actions[2].serviceId", "service 1 belongs to client 1")
	assert.Contains(t, e.Error.Details, "actions[3].amountCents")

	code, _, _ = postWhatIf(t, h, cookie, "/api/v1/customers/5/what-if", `{"actions":[]}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _, _ = postWhatIf(t, h, cookie, "/api/v1/customers/999/what-if", `{"actions":[{"type":"pay_waiting"}]}`)
	assert.Equal(t, http.StatusNotFound, code)
	code, _, _ = postWhatIf(t, h, nil, "/api/v1/customers/5/what-if", `{"actions":[{"type":"pay_waiting"}]}`)
	assert.Equal(t, http.StatusUnauthorized, code)
}
