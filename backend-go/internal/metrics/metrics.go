// Package metrics exposes the risk model to Prometheus (docs/ОБЩЕЕ_ТЗ.md §7.11).
//
// The numbers are not copied into gauges on every recalculation: Collect reads the published
// Snapshot at scrape time, so /metrics always shows the same calculation as the API.
package metrics

import (
	"context"
	"math"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
)

// Source is where the numbers come from (*recalc.Worker).
type Source interface {
	Current() *recalc.Snapshot // nil before the first calculation
	Errors() int64             // failed calculations since start
}

var (
	customersDesc = prometheus.NewDesc("prolongo_customers",
		"Customers by risk level (none: no score)", []string{"level"}, nil)
	expectedLossDesc = prometheus.NewDesc("prolongo_expected_loss_eur",
		"Expected loss over the horizon, EUR", nil, nil)
	worstCaseDesc = prometheus.NewDesc("prolongo_worst_case_loss_eur",
		"Loss over the horizon if every high-risk customer leaves, EUR", nil, nil)
	mrrAtRiskDesc = prometheus.NewDesc("prolongo_mrr_at_risk_eur",
		"Monthly revenue of medium and high risk customers, EUR", nil, nil)
	riskScoreAvgDesc = prometheus.NewDesc("prolongo_risk_score_avg",
		"Average risk score of scored customers, 0..100", nil, nil)
	servicesDesc = prometheus.NewDesc("prolongo_services",
		"Services by status", []string{"status"}, nil)
	servicesAtRiskDesc = prometheus.NewDesc("prolongo_services_at_risk",
		"Live services with a score at or above the medium threshold", nil, nil)
	calcVersionDesc = prometheus.NewDesc("prolongo_calc_version",
		"Version of the published calculation, grows with every recalculation", nil, nil)
	lastRecalcDesc = prometheus.NewDesc("prolongo_last_recalc_timestamp_seconds",
		"When the published calculation finished, Unix time", nil, nil)
	featureWeightDesc = prometheus.NewDesc("prolongo_model_feature_weight",
		"Effective weight of a model feature, 0 when disabled", []string{"feature"}, nil)
	recalcErrorsDesc = prometheus.NewDesc("prolongo_recalc_errors_total",
		"Failed recalculations since start; the previous result stays published", nil, nil)
	sseClientsDesc = prometheus.NewDesc("prolongo_sse_clients",
		"Panels connected to the live stream", nil, nil)
)

// Every service status, so each series exists even when its count is 0:
// a series that appears and disappears breaks graphs and alerts.
var serviceStatuses = []domain.ServiceStatus{
	domain.StatusActive, domain.StatusProlongWait, domain.StatusSuspended,
	domain.StatusBlocked, domain.StatusDeleted,
}

var recalcReasons = []string{recalc.ReasonStartup, recalc.ReasonData, recalc.ReasonSettings, recalc.ReasonTimer}

// Collector is a prometheus.Collector over the latest calculation.
type Collector struct {
	src      Source
	clients  func() int // SSE clients (*live.Hub.Count); nil leaves the metric out
	duration *prometheus.HistogramVec
}

// New builds the collector. clients may be nil.
func New(src Source, clients func() int) *Collector {
	c := &Collector{
		src:     src,
		clients: clients,
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "prolongo_recalc_duration_seconds",
			Help: "Duration of successful recalculations by reason",
			// 10 customers take about a millisecond; the generator's thousands take longer.
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"reason"}),
	}
	for _, r := range recalcReasons {
		c.duration.WithLabelValues(r) // the series start at 0 instead of appearing later
	}
	return c
}

// Observe records a published calculation in the duration histogram. Register it with
// worker.OnPublish: unlike the gauges, a histogram must see every calculation, not just the last.
func (c *Collector) Observe(_ context.Context, snap *recalc.Snapshot) {
	c.duration.WithLabelValues(snap.Reason).Observe(snap.Duration.Seconds())
}

// Describe sends the descriptors of every metric (prometheus.Collector).
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		customersDesc, expectedLossDesc, worstCaseDesc, mrrAtRiskDesc, riskScoreAvgDesc,
		servicesDesc, servicesAtRiskDesc, calcVersionDesc, lastRecalcDesc, featureWeightDesc,
		recalcErrorsDesc, sseClientsDesc,
	} {
		ch <- d
	}
	c.duration.Describe(ch)
}

// Collect is called on every scrape (prometheus.Collector). It reads the Snapshot once,
// so all values in one response come from the same calculation.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(recalcErrorsDesc, prometheus.CounterValue, float64(c.src.Errors()))
	if c.clients != nil {
		ch <- gauge(sseClientsDesc, float64(c.clients()))
	}
	c.duration.Collect(ch)

	snap := c.src.Current()
	if snap == nil {
		return // nothing calculated yet: no numbers rather than zeros that look real
	}
	s := snap.Summary
	ch <- gauge(customersDesc, float64(s.Customers.High), engine.LevelHigh)
	ch <- gauge(customersDesc, float64(s.Customers.Medium), engine.LevelMedium)
	ch <- gauge(customersDesc, float64(s.Customers.Low), engine.LevelLow)
	ch <- gauge(customersDesc, float64(s.Customers.NoScore), "none")

	ch <- gauge(expectedLossDesc, euros(s.Money.ExpectedLoss))
	ch <- gauge(worstCaseDesc, euros(s.Money.WorstCase))
	ch <- gauge(mrrAtRiskDesc, euros(s.Money.MRRAtRisk))
	ch <- gauge(riskScoreAvgDesc, riskScoreAvg(snap.Customers))

	byStatus := map[domain.ServiceStatus]int{}
	for i := range snap.Customers {
		for _, sr := range snap.Customers[i].Services {
			byStatus[sr.Service.Status]++
		}
	}
	for _, st := range serviceStatuses {
		ch <- gauge(servicesDesc, float64(byStatus[st]), string(st))
	}
	ch <- gauge(servicesAtRiskDesc, float64(s.Services.AtRisk))

	ch <- gauge(calcVersionDesc, float64(snap.Version))
	ch <- gauge(lastRecalcDesc, float64(snap.CalculatedAt.UnixMilli())/1000)

	for _, f := range features.All() {
		m := f.Meta()
		fs := snap.Settings.Feature(m)
		w := fs.Weight
		if !fs.Enabled {
			w = 0
		}
		ch <- gauge(featureWeightDesc, w, m.Key)
	}
}

// Handler serves /metrics: the model plus the standard go_* and process_* metrics.
// A registry of our own (not the global default) keeps tests independent of each other.
func Handler(c *Collector) http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(c, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

func gauge(d *prometheus.Desc, v float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
}

// euros turns cents into EUR (docs/ТЗ_BACKEND_GO.md §14). Prometheus prints the shortest
// decimal (703267 → 7032.67); round(value × 100) always gives the API's cents back.
func euros(cents int64) float64 {
	return float64(cents) / 100
}

// riskScoreAvg averages the shown (rounded) scores; customers without a score are left out.
func riskScoreAvg(customers []engine.CustomerResult) float64 {
	var sum, n int
	for i := range customers {
		if customers[i].ScoreStatus == engine.StatusScored {
			sum += customers[i].RiskScore
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return math.Round(float64(sum)/float64(n)*100) / 100
}
