package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// G8.1 acceptance: the period switcher at 7 days gives 89 738 cents of expected loss.
func TestSummaryOtherHorizon(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	code, s7 := getJSON[summaryJSON](t, h, cookie, "/api/v1/analytics/summary?horizonDays=7")
	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 89738, s7.Money.ExpectedLossCents)
	assert.Equal(t, 7, s7.HorizonDays)
	assert.Equal(t, [3]int{1, 3, 2}, [3]int{s7.Customers.High, s7.Customers.Medium, s7.Customers.Low})
	assert.EqualValues(t, 17, s7.CalcVersion, "same calculation, another view")

	_, s30 := getJSON[summaryJSON](t, h, cookie, "/api/v1/analytics/summary?horizonDays=30")
	assert.EqualValues(t, 703267, s30.Money.ExpectedLossCents, "the saved horizon is the published result")

	// Settings did not change.
	_, settings := getJSON[modelSettingsJSON](t, h, cookie, "/api/v1/model/settings")
	assert.Equal(t, 30, settings.HorizonDays)
	assert.EqualValues(t, 1, settings.Version)
}

func TestListAndCardOtherHorizon(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	_, list := getJSON[customerListJSON](t, h, cookie, "/api/v1/customers?horizonDays=90&pageSize=2")
	require.Len(t, list.Items, 2)
	var total int64
	_, all := getJSON[customerListJSON](t, h, cookie, "/api/v1/customers?horizonDays=90")
	for _, it := range all.Items {
		total += it.ExpectedLossCents
	}
	assert.InDelta(t, 2378403, total, 2, "90 days")

	_, card := getJSON[customerDetailJSON](t, h, cookie, "/api/v1/customers/5?horizonDays=14")
	assert.Equal(t, 14, card.Money.HorizonDays)
	assert.Contains(t, card.Money.Explanation, "за 14 дней")
}

func TestHorizonValidation(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)
	for _, path := range []string{
		"/api/v1/analytics/summary?horizonDays=0",
		"/api/v1/analytics/summary?horizonDays=366",
		"/api/v1/customers?horizonDays=week",
		"/api/v1/customers/5?horizonDays=-1",
	} {
		rec := do(h, http.MethodGet, path, "", cookie)
		assert.Equal(t, http.StatusBadRequest, rec.Code, path)
		assert.JSONEq(t, `{"error":{"code":"bad_request","message":"horizonDays: целое число от 1 до 365"}}`, rec.Body.String(), path)
	}
}

func TestHorizonCache(t *testing.T) {
	base := referenceSnapshot(t).snap
	hd := &handlers{horizons: &horizonCache{}}

	assert.Same(t, base.Result, hd.resultFor(base, 0), "no parameter: the published result")
	assert.Same(t, base.Result, hd.resultFor(base, 30), "the saved horizon: no recalculation")
	assert.Zero(t, hd.horizons.len())

	r7 := hd.resultFor(base, 7)
	assert.Same(t, r7, hd.resultFor(base, 7), "second request is served from the cache")
	assert.Equal(t, 1, hd.horizons.len())

	for days := 31; days <= 40; days++ { // ten other horizons, none of them 7
		hd.resultFor(base, days)
	}
	assert.Equal(t, horizonCacheSize, hd.horizons.len(), "at most 8 horizons")
	assert.NotSame(t, r7, hd.resultFor(base, 7), "7 was evicted (least recently used) and recalculated")

	// A new calculation (data or settings changed) makes every cached horizon stale.
	next := *base
	next.Version = base.Version + 1
	fresh := hd.resultFor(&next, 7)
	assert.NotSame(t, r7, fresh)
	assert.Equal(t, 1, hd.horizons.len(), "the cache started over for the new calcVersion")

	// A slow request for the old calculation must not bring stale entries back.
	hd.horizons.put(base.Version, 60, r7)
	assert.Equal(t, 1, hd.horizons.len())
}
