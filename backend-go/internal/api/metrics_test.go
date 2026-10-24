package api

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMetricsToken = "test-metrics-token"

func getMetrics(h http.Handler, authorization string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMetricsBearer(t *testing.T) {
	h := NewRouter(testDeps(t, nil))

	for _, header := range []string{"Bearer " + testMetricsToken, "bearer " + testMetricsToken} {
		rec := getMetrics(h, header, nil)
		require.Equal(t, http.StatusOK, rec.Code, header)
		assert.Contains(t, rec.Body.String(), "prolongo_expected_loss_eur 7032.67")
	}

	for _, header := range []string{
		"Bearer wrong", "Bearer " + testMetricsToken + "x", "Bearer " + testMetricsToken[:5], "Bearer ",
	} {
		rec := getMetrics(h, header, nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, header)
		assert.JSONEq(t, `{"error":{"code":"unauthorized","message":"Неверный токен метрик"}}`, rec.Body.String())
	}
}

// A wrong token is refused even with a valid session: the request clearly meant the token.
func TestMetricsWrongBearerWithSession(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	assert.Equal(t, http.StatusUnauthorized, getMetrics(h, "Bearer wrong", login(t, h)).Code)
}

func TestMetricsSession(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	assert.Equal(t, http.StatusUnauthorized, getMetrics(h, "", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, getMetrics(h, "Basic YWRtaW46YWRtaW4xMjM=", nil).Code, "only Bearer is a token")

	rec := getMetrics(h, "", login(t, h))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `prolongo_customers{level="high"} 2`)
}

// Without METRICS_TOKEN no token opens /metrics, not even an empty one.
func TestMetricsWithoutToken(t *testing.T) {
	d := testDeps(t, nil)
	d.MetricsToken = ""
	h := NewRouter(d)
	assert.Equal(t, http.StatusUnauthorized, getMetrics(h, "Bearer ", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, getMetrics(h, "Bearer "+testMetricsToken, nil).Code)
	assert.Equal(t, http.StatusOK, getMetrics(h, "", login(t, h)).Code)
}

func TestMetricsNotMounted(t *testing.T) {
	d := testDeps(t, nil)
	d.Metrics = nil
	assert.Equal(t, http.StatusNotFound, getMetrics(NewRouter(d), "Bearer "+testMetricsToken, nil).Code)
}

// G10 acceptance: prolongo_expected_loss_eur × 100 = summary.money.expectedLossCents.
func TestMetricsMatchSummary(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	_, s := getJSON[summaryJSON](t, h, login(t, h), "/api/v1/analytics/summary")

	body := getMetrics(h, "Bearer "+testMetricsToken, nil).Body.String()
	assert.Equal(t, s.Money.ExpectedLossCents, cents(t, body, "prolongo_expected_loss_eur"))
	assert.Equal(t, s.Money.WorstCaseLossCents, cents(t, body, "prolongo_worst_case_loss_eur"))
	assert.Equal(t, s.Money.MRRAtRiskCents, cents(t, body, "prolongo_mrr_at_risk_eur"))
	assert.Contains(t, body, "prolongo_calc_version "+strconv.FormatInt(s.CalcVersion, 10)+"\n")
}

func cents(t *testing.T, body, name string) int64 {
	t.Helper()
	for line := range strings.Lines(body) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), name+" "); ok {
			eur, err := strconv.ParseFloat(v, 64)
			require.NoError(t, err)
			return int64(math.Round(eur * 100))
		}
	}
	t.Fatalf("%s not found", name)
	return 0
}
