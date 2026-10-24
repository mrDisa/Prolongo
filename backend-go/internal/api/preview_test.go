package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

func postPreview(t *testing.T, h http.Handler, cookie *http.Cookie, body string) (int, previewJSON, string) {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/model/preview", body, cookie)
	var p previewJSON
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	}
	return rec.Code, p, rec.Body.String()
}

// G8.1 acceptance: a draft with thresholds.high = 40 → high 3, medium 1.
func TestPreviewThresholds(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	code, p, _ := postPreview(t, h, cookie, `{"thresholds":{"high":40}}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, [4]int{3, 1, 2, 4}, [4]int{p.Summary.Customers.High, p.Summary.Customers.Medium, p.Summary.Customers.Low, p.Summary.Customers.NoScore})
	assert.Equal(t, [4]int{2, 2, 2, 4}, [4]int{p.Baseline.Customers.High, p.Baseline.Customers.Medium, p.Baseline.Customers.Low, p.Baseline.Customers.NoScore})
	assert.EqualValues(t, 703267, p.Summary.Money.ExpectedLossCents, "thresholds do not change money")
	assert.EqualValues(t, 966052, p.Baseline.Money.WorstCaseLossCents)

	require.Len(t, p.Customers, 6, "only customers with a score")
	assert.Equal(t, previewCustomerJSON{
		ID: 5, Email: "user-0005@example.test", RiskScore: 77, RiskScoreRaw: 76.7, RiskLevel: "high",
		ExpectedLossCents: 338372, RiskiestServiceName: "service-0025",
		Contributions: []previewPointsJSON{{"prolong_wait", 45}, {"insufficient_funds", 28.2}, {"loyal_payer", -10},
			{"new_client", 5.5}, {"short_billing", 5}, {"failed_payments", 3}},
	}, p.Customers[0], "riskiest first")
	for _, c := range p.Customers {
		if c.ID == 10 {
			assert.Equal(t, "high", c.RiskLevel, "41 ≥ 40")
		}
	}

	require.Len(t, p.Features, len(features.All()), "every feature, even ones that hit nobody")
	byKey := map[string]previewFeatureJSON{}
	for _, f := range p.Features {
		byKey[f.Key] = f
	}
	assert.Equal(t, previewFeatureJSON{Key: "prolong_wait", CustomersHit: 2, MaxPoints: 45}, byKey["prolong_wait"])
	assert.Equal(t, previewFeatureJSON{Key: "churn_history"}, byKey["churn_history"], "nobody has deleted services among scored customers")
	assert.EqualValues(t, -10, byKey["loyal_payer"].MaxPoints, "negative features keep their sign")
}

func TestPreviewWeightsAndHorizon(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	_, same, _ := postPreview(t, h, cookie, `{}`)
	assert.Equal(t, same.Baseline, same.Summary, "an empty draft changes nothing")

	_, heavier, _ := postPreview(t, h, cookie, `{"features":{"prolong_wait":{"weight":60}}}`)
	assert.Greater(t, heavier.Summary.Money.ExpectedLossCents, heavier.Baseline.Money.ExpectedLossCents)

	_, h7, _ := postPreview(t, h, cookie, `{"horizonDays":7}`)
	assert.EqualValues(t, 89738, h7.Summary.Money.ExpectedLossCents)
}

func TestPreviewSavesNothing(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	code, _, _ := postPreview(t, h, cookie, `{"horizonDays":7,"thresholds":{"high":40}}`)
	require.Equal(t, http.StatusOK, code)

	_, s := getJSON[modelSettingsJSON](t, h, cookie, "/api/v1/model/settings")
	assert.EqualValues(t, 1, s.Version)
	assert.Equal(t, 30, s.HorizonDays)
	assert.Equal(t, 60, s.Thresholds.High)
}

func TestPreviewValidation(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	code, _, body := postPreview(t, h, cookie, `{"thresholds":{"medium":70}}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	assert.JSONEq(t, `{"error":{"code":"validation_error","message":"Настройки не прошли проверку",
		"details":{"thresholds.medium":["Должен быть меньше high (60)"]}}}`, body)

	code, _, body = postPreview(t, h, cookie, `{"horizon":7}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	assert.Contains(t, body, `"body"`, "unknown fields are rejected like in PATCH")
}
