package metrics

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
)

type fakeSource struct {
	snap   *recalc.Snapshot
	errors int64
}

func (f *fakeSource) Current() *recalc.Snapshot { return f.snap }
func (f *fakeSource) Errors() int64             { return f.errors }

// reference calculates the organizers' data.json with the given settings.
func reference(t *testing.T, s engine.Settings) *recalc.Snapshot {
	t.Helper()
	f, err := os.Open("../../../data/test-data/data.json")
	require.NoError(t, err)
	defer f.Close()
	ds, err := domain.FromJSON(f)
	require.NoError(t, err)
	return &recalc.Snapshot{
		Result:       engine.Calculate(ds, s, engine.Now(s.Clock, ds, time.Now())),
		Version:      17,
		CalculatedAt: time.Date(2026, 10, 19, 14, 35, 54, 204_000_000, time.UTC),
		Reason:       recalc.ReasonStartup,
		Duration:     1200 * time.Microsecond,
	}
}

// The model metrics on data.json: the reference numbers of docs/ОБЩЕЕ_ТЗ.md §4.2.1.
func TestReferenceValues(t *testing.T) {
	c := New(&fakeSource{snap: reference(t, engine.DefaultSettings())}, nil)

	expected := `
# HELP prolongo_customers Customers by risk level (none: no score)
# TYPE prolongo_customers gauge
prolongo_customers{level="high"} 2
prolongo_customers{level="medium"} 2
prolongo_customers{level="low"} 2
prolongo_customers{level="none"} 4
# HELP prolongo_expected_loss_eur Expected loss over the horizon, EUR
# TYPE prolongo_expected_loss_eur gauge
prolongo_expected_loss_eur 7032.67
# HELP prolongo_worst_case_loss_eur Loss over the horizon if every high-risk customer leaves, EUR
# TYPE prolongo_worst_case_loss_eur gauge
prolongo_worst_case_loss_eur 9660.52
# HELP prolongo_mrr_at_risk_eur Monthly revenue of medium and high risk customers, EUR
# TYPE prolongo_mrr_at_risk_eur gauge
prolongo_mrr_at_risk_eur 11555.77
# HELP prolongo_risk_score_avg Average risk score of scored customers, 0..100
# TYPE prolongo_risk_score_avg gauge
prolongo_risk_score_avg 36.5
# HELP prolongo_calc_version Version of the published calculation, grows with every recalculation
# TYPE prolongo_calc_version gauge
prolongo_calc_version 17
# HELP prolongo_last_recalc_timestamp_seconds When the published calculation finished, Unix time
# TYPE prolongo_last_recalc_timestamp_seconds gauge
prolongo_last_recalc_timestamp_seconds 1.792420554204e+09
`
	require.NoError(t, testutil.CollectAndCompare(c, strings.NewReader(expected),
		"prolongo_customers", "prolongo_expected_loss_eur", "prolongo_worst_case_loss_eur",
		"prolongo_mrr_at_risk_eur", "prolongo_risk_score_avg",
		"prolongo_calc_version", "prolongo_last_recalc_timestamp_seconds"))
}

// Services: every status is present, the counts add up to the summary.
func TestServices(t *testing.T) {
	snap := reference(t, engine.DefaultSettings())
	reg := gather(t, New(&fakeSource{snap: snap}, nil))

	total := 0.0
	for _, st := range serviceStatuses {
		v, ok := reg[`prolongo_services{status="`+string(st)+`"}`]
		require.True(t, ok, "series for %s", st)
		total += v
	}
	assert.EqualValues(t, snap.Summary.Services.Total, total)
	assert.EqualValues(t, snap.Summary.Services.ProlongWait, reg[`prolongo_services{status="prolong_wait"}`])
	assert.EqualValues(t, snap.Summary.Services.AtRisk, reg["prolongo_services_at_risk"])
	assert.Positive(t, reg["prolongo_services_at_risk"])
}

// Feature weights follow the saved settings; a disabled feature reports 0.
func TestFeatureWeights(t *testing.T) {
	s := engine.DefaultSettings()
	all := features.All()
	require.GreaterOrEqual(t, len(all), 2)
	first, second := all[0].Meta(), all[1].Meta()
	s.Features[first.Key] = engine.FeatureSetting{Weight: 7, Enabled: true}
	s.Features[second.Key] = engine.FeatureSetting{Weight: second.DefaultWeight, Enabled: false}

	reg := gather(t, New(&fakeSource{snap: reference(t, s)}, nil))
	for _, f := range all {
		_, ok := reg[`prolongo_model_feature_weight{feature="`+f.Meta().Key+`"}`]
		assert.True(t, ok, "every registered feature has a series: %s", f.Meta().Key)
	}
	assert.EqualValues(t, 7, reg[`prolongo_model_feature_weight{feature="`+first.Key+`"}`])
	assert.Zero(t, reg[`prolongo_model_feature_weight{feature="`+second.Key+`"}`])
}

// Before the first calculation there are no model numbers: zeros would look like real data.
func TestNoCalculationYet(t *testing.T) {
	reg := gather(t, New(&fakeSource{errors: 3}, func() int { return 2 }))
	assert.EqualValues(t, 3, reg["prolongo_recalc_errors_total"])
	assert.EqualValues(t, 2, reg["prolongo_sse_clients"])
	for name := range reg {
		assert.False(t, strings.HasPrefix(name, "prolongo_customers") || strings.HasPrefix(name, "prolongo_expected_loss"), name)
	}
}

// The histogram counts every published calculation by reason.
func TestRecalcDuration(t *testing.T) {
	c := New(&fakeSource{}, nil)
	assert.Equal(t, len(recalcReasons), testutil.CollectAndCount(c, "prolongo_recalc_duration_seconds"),
		"a series per reason from the start")

	ctx := context.Background()
	c.Observe(ctx, &recalc.Snapshot{Reason: recalc.ReasonData, Duration: 2 * time.Millisecond})
	c.Observe(ctx, &recalc.Snapshot{Reason: recalc.ReasonData, Duration: 3 * time.Millisecond})
	c.Observe(ctx, &recalc.Snapshot{Reason: recalc.ReasonSettings, Duration: 40 * time.Millisecond})

	reg := gather(t, c)
	assert.EqualValues(t, 2, reg[`prolongo_recalc_duration_seconds_count{reason="data"}`])
	assert.InDelta(t, 0.005, reg[`prolongo_recalc_duration_seconds_sum{reason="data"}`], 1e-9)
	assert.EqualValues(t, 1, reg[`prolongo_recalc_duration_seconds_count{reason="settings"}`])
	assert.EqualValues(t, 1, reg[`prolongo_recalc_duration_seconds_bucket{reason="data",le="0.0025"}`])
	assert.Zero(t, reg[`prolongo_recalc_duration_seconds_count{reason="timer"}`])
}

// EUR in metrics × 100 gives the API's cents back for any amount.
func TestEurosRoundTrip(t *testing.T) {
	for _, cents := range []int64{0, 1, 29, 99, 89738, 703267, 966052, 1155577, 2378403, 12345678901} {
		assert.Equal(t, cents, int64(math.Round(euros(cents)*100)), cents)
	}
}

// Prometheus' own linter: names, units, HELP texts.
func TestLint(t *testing.T) {
	c := New(&fakeSource{snap: reference(t, engine.DefaultSettings())}, func() int { return 0 })
	problems, err := testutil.CollectAndLint(c)
	require.NoError(t, err)
	assert.Empty(t, problems)
}

// The handler adds the standard Go runtime and process metrics.
func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler(New(&fakeSource{snap: reference(t, engine.DefaultSettings())}, nil)))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain"), resp.Header.Get("Content-Type"))
	for _, want := range []string{"prolongo_expected_loss_eur 7032.67", "go_goroutines", "process_start_time_seconds"} {
		assert.Contains(t, string(body), want)
	}
}

// gather collects every sample into "name{labels}" → value.
func gather(t *testing.T, c *Collector) map[string]float64 {
	t.Helper()
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return parseText(t, string(body))
}

// parseText reads the text exposition format: "name{labels} value" per line, "#" lines skipped.
func parseText(t *testing.T, body string) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for line := range strings.Lines(body) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.LastIndexByte(line, ' ')
		require.Positive(t, i, line)
		v, err := strconv.ParseFloat(line[i+1:], 64)
		require.NoError(t, err, line)
		out[line[:i]] = v
	}
	return out
}
