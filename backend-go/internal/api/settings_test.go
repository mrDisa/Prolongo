package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

func decodeSettings(t *testing.T, body []byte) modelSettingsJSON {
	t.Helper()
	var s modelSettingsJSON
	require.NoError(t, json.Unmarshal(body, &s))
	return s
}

func TestGetSettings(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodGet, "/api/v1/model/settings", "", login(t, h))
	require.Equal(t, http.StatusOK, rec.Code)

	s := decodeSettings(t, rec.Body.Bytes())
	assert.EqualValues(t, 1, s.Version)
	assert.Equal(t, 30, s.HorizonDays)
	assert.Equal(t, thresholdsJSON{Medium: 30, High: 60}, s.Thresholds)
	assert.Equal(t, clockJSON{Mode: "data"}, s.Clock)
	require.Len(t, s.Features, len(features.All()), "every registered feature, nothing to update when one is added")

	var pw featureSettingJSON
	for _, f := range s.Features {
		if f.Key == "prolong_wait" {
			pw = f
		}
	}
	assert.Equal(t, featureSettingJSON{
		Key: "prolong_wait", Title: "Ожидает оплаты", Description: "Услуга в статусе prolong_wait: срок истёк, через 5 дней будет удалена.",
		Scope: "service", Weight: 45, DefaultWeight: 45, Min: 0, Max: 100, Enabled: true,
	}, pw)

	// Raw JSON keeps the contract's field names, including null fixedAt.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.Contains(t, raw["clock"], "fixedAt")
	for _, key := range []string{"version", "updatedAt", "horizonDays", "thresholds", "clock", "features"} {
		assert.Contains(t, raw, key)
	}
}

// Every save and reset is a version; the list is newest first and each item is full settings.
func TestSettingsVersions(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	for _, req := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/v1/model/settings", `{"features":{"prolong_wait":{"weight":60}}}`},
		{http.MethodPatch, "/api/v1/model/settings", `{"thresholds":{"high":70}}`},
		{http.MethodPatch, "/api/v1/model/settings", `{"thresholds":{"high":0}}`}, // 422: not a version
		{http.MethodPost, "/api/v1/model/settings/reset", ``},
	} {
		do(h, req.method, req.path, req.body, cookie)
	}

	rec := do(h, http.MethodGet, "/api/v1/model/settings/versions", "", cookie)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out settingsVersionsJSON
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 4)

	versions := make([]int64, 0, len(out.Items))
	for _, s := range out.Items {
		versions = append(versions, s.Version)
		require.Len(t, s.Features, len(features.All()))
	}
	assert.Equal(t, []int64{4, 3, 2, 1}, versions)

	current := decodeSettings(t, do(h, http.MethodGet, "/api/v1/model/settings", "", cookie).Body.Bytes())
	assert.Equal(t, current, out.Items[0], "the first item is the current settings")
	assert.Equal(t, 70, out.Items[1].Thresholds.High)
	weight := func(s modelSettingsJSON) float64 {
		for _, f := range s.Features {
			if f.Key == "prolong_wait" {
				return f.Weight
			}
		}
		return 0
	}
	assert.EqualValues(t, 60, weight(out.Items[2]), "an old version keeps its own weights")
	assert.EqualValues(t, 45, weight(out.Items[3]))
}

func TestPatchSettings(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/model/settings",
		`{"horizonDays":14,"thresholds":{"high":65},"clock":{"mode":"real"},"features":{"prolong_wait":{"weight":50,"enabled":true}}}`, cookie)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	s := decodeSettings(t, rec.Body.Bytes())
	assert.EqualValues(t, 2, s.Version)
	assert.Equal(t, 14, s.HorizonDays)
	assert.Equal(t, thresholdsJSON{Medium: 30, High: 65}, s.Thresholds)
	assert.Equal(t, "real", s.Clock.Mode)

	// GET sees the saved values.
	s = decodeSettings(t, do(h, http.MethodGet, "/api/v1/model/settings", "", cookie).Body.Bytes())
	assert.Equal(t, 14, s.HorizonDays)
	for _, f := range s.Features {
		if f.Key == "prolong_wait" {
			assert.EqualValues(t, 50, f.Weight)
			assert.EqualValues(t, 45, f.DefaultWeight)
		}
	}
}

func TestPatchSettingsValidation(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	rec := do(h, http.MethodPatch, "/api/v1/model/settings",
		`{"thresholds":{"medium":70},"features":{"prolong_wait":{"weight":150}}}`, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"validation_error","message":"Настройки не прошли проверку","details":{
		"thresholds.medium":["Должен быть меньше high (60)"],
		"features.prolong_wait.weight":["Допустимо от 0 до 100"]}}}`, rec.Body.String())

	s := decodeSettings(t, do(h, http.MethodGet, "/api/v1/model/settings", "", cookie).Body.Bytes())
	assert.EqualValues(t, 1, s.Version, "nothing saved")
}

func TestPatchSettingsBadBody(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	for name, body := range map[string]string{
		"typo in a field": `{"horizon":14}`,
		"wrong type":      `{"horizonDays":"14"}`,
		"not json":        `horizonDays=14`,
	} {
		rec := do(h, http.MethodPatch, "/api/v1/model/settings", body, cookie)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, name)
		assert.Contains(t, rec.Body.String(), `"body"`, name)
	}
}

func TestResetSettings(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	do(h, http.MethodPatch, "/api/v1/model/settings", `{"horizonDays":90}`, cookie)

	rec := do(h, http.MethodPost, "/api/v1/model/settings/reset", "", cookie)
	require.Equal(t, http.StatusOK, rec.Code)
	s := decodeSettings(t, rec.Body.Bytes())
	assert.EqualValues(t, 3, s.Version)
	assert.Equal(t, 30, s.HorizonDays)
}
