package api

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

// SnapshotSource gives the latest calculation (*recalc.Worker).
type SnapshotSource interface {
	Current() *recalc.Snapshot
}

// HistorySource reads status changes of a customer (*store.Store).
type HistorySource interface {
	StatusHistory(ctx context.Context, accountID int64, limit int) ([]store.StatusEvent, error)
}

const statusHistoryLimit = 50

// listQuery is a validated GET /customers query.
type listQuery struct {
	sort     string
	desc     bool
	levels   map[string]bool // nil = every level; "none" = customers without a score
	search   string
	page     int
	pageSize int
}

var sortFields = []string{"id", "email", "balance", "totalReplenished", "riskScore", "expectedLoss"}

// parseListQuery validates the query; the error text goes to the 400 response as is.
func parseListQuery(r *http.Request) (listQuery, error) {
	v := r.URL.Query()
	q := listQuery{sort: "expectedLoss", desc: true, page: 1, pageSize: 25, search: strings.TrimSpace(v.Get("search"))}

	if s := v.Get("sort"); s != "" {
		if !slices.Contains(sortFields, s) {
			return q, errors.New("sort: допустимые значения " + strings.Join(sortFields, ", "))
		}
		q.sort = s
	}
	switch v.Get("order") {
	case "", "desc":
	case "asc":
		q.desc = false
	default:
		return q, errors.New("order: допустимые значения asc, desc")
	}
	if l := v.Get("level"); l != "" {
		q.levels = map[string]bool{}
		for _, part := range strings.Split(l, ",") {
			part = strings.TrimSpace(part)
			if !slices.Contains([]string{engine.LevelHigh, engine.LevelMedium, engine.LevelLow, "none"}, part) {
				return q, errors.New("level: допустимые значения high, medium, low, none через запятую")
			}
			q.levels[part] = true
		}
	}
	var err error
	if q.page, err = intParam(v.Get("page"), 1, 1, 1<<30); err != nil {
		return q, errors.New("page: целое число от 1")
	}
	if q.pageSize, err = intParam(v.Get("pageSize"), 25, 1, 200); err != nil {
		return q, errors.New("pageSize: целое число от 1 до 200")
	}
	return q, nil
}

func intParam(s string, def, lo, hi int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		return 0, errors.New("out of range")
	}
	return n, nil
}

// matches applies the level filter and the search (docs/ТЗ_BACKEND_GO.md §12.2): a numeric
// search matches the exact id OR a substring of the email; otherwise a case-insensitive substring.
func (q listQuery) matches(cr *engine.CustomerResult) bool {
	if q.levels != nil {
		level := cr.Level
		if cr.ScoreStatus != engine.StatusScored {
			level = "none"
		}
		if !q.levels[level] {
			return false
		}
	}
	if q.search == "" {
		return true
	}
	if id, err := strconv.ParseInt(q.search, 10, 64); err == nil && id == cr.Account.ID {
		return true
	}
	return strings.Contains(strings.ToLower(cr.Account.Email), strings.ToLower(q.search))
}

// compare orders two customers for the chosen field and direction. Ties go by id asc
// (stable pages); customers without a score always come last when sorting by risk or money.
func (q listQuery) compare(a, b *engine.CustomerResult) int {
	if q.sort == "riskScore" || q.sort == "expectedLoss" {
		aScored, bScored := a.ScoreStatus == engine.StatusScored, b.ScoreStatus == engine.StatusScored
		if aScored != bScored {
			if aScored {
				return -1
			}
			return 1
		}
	}
	var c int
	switch q.sort {
	case "id":
		c = cmp.Compare(a.Account.ID, b.Account.ID)
	case "email":
		c = strings.Compare(strings.ToLower(a.Account.Email), strings.ToLower(b.Account.Email))
	case "balance":
		c = cmp.Compare(a.Account.Balance, b.Account.Balance)
	case "totalReplenished":
		c = cmp.Compare(a.Account.TotalReplenished, b.Account.TotalReplenished)
	case "riskScore":
		c = cmp.Compare(a.RiskScoreRaw, b.RiskScoreRaw)
	case "expectedLoss":
		c = cmp.Compare(a.Money.ExpectedLoss, b.Money.ExpectedLoss)
	}
	if q.desc {
		c = -c
	}
	if c != 0 {
		return c
	}
	return cmp.Compare(a.Account.ID, b.Account.ID)
}

// snapshot returns the current calculation or writes 500: after startup there is always one.
func (h *handlers) snapshot(w http.ResponseWriter, r *http.Request) (*recalc.Snapshot, bool) {
	snap := h.current.Current()
	if snap == nil {
		writeInternal(w, r, h.logger, errors.New("no calculation yet"))
		return nil, false
	}
	return snap, true
}

// listCustomers: GET /api/v1/customers.
func (h *handlers) listCustomers(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error(), nil)
		return
	}
	snap, ok := h.snapshot(w, r)
	if !ok {
		return
	}

	var found []*engine.CustomerResult
	for i := range snap.Customers {
		if cr := &snap.Customers[i]; q.matches(cr) {
			found = append(found, cr)
		}
	}
	slices.SortStableFunc(found, q.compare)

	out := customerListJSON{
		Items: []customerListItemJSON{}, Total: len(found), Page: q.page, PageSize: q.pageSize,
		CalcVersion: snap.Version, CalculatedAt: snap.CalculatedAt,
	}
	from := min((q.page-1)*q.pageSize, len(found))
	to := min(from+q.pageSize, len(found))
	for _, cr := range found[from:to] {
		out.Items = append(out.Items, toListItem(cr))
	}
	writeJSON(w, http.StatusOK, out)
}

// getCustomer: GET /api/v1/customers/{id}.
func (h *handlers) getCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, codeBadRequest, "id: целое число от 1", nil)
		return
	}
	snap, ok := h.snapshot(w, r)
	if !ok {
		return
	}
	cr, found := snap.Customer(id)
	if !found {
		writeError(w, http.StatusNotFound, codeNotFound, "Клиент не найден", nil)
		return
	}
	history, err := h.history.StatusHistory(r.Context(), id, statusHistoryLimit)
	if err != nil {
		writeInternal(w, r, h.logger, err)
		return
	}
	d := toDetail(cr, snap.Dataset.TxByService, history)
	d.CalcVersion, d.CalculatedAt = snap.Version, snap.CalculatedAt
	writeJSON(w, http.StatusOK, d)
}

// summary: GET /api/v1/analytics/summary.
func (h *handlers) summary(w http.ResponseWriter, r *http.Request) {
	snap, ok := h.snapshot(w, r)
	if !ok {
		return
	}
	s := snap.Summary
	var out summaryJSON
	out.CalcVersion, out.CalculatedAt = snap.Version, snap.CalculatedAt
	out.ReferenceTime, out.ClockMode, out.HorizonDays = snap.Now, snap.ClockMode, snap.HorizonDays
	out.Customers.Total, out.Customers.High, out.Customers.Medium = s.Customers.Total, s.Customers.High, s.Customers.Medium
	out.Customers.Low, out.Customers.NoScore = s.Customers.Low, s.Customers.NoScore
	out.Services.Total, out.Services.Live = s.Services.Total, s.Services.Live
	out.Services.AtRisk, out.Services.ProlongWait = s.Services.AtRisk, s.Services.ProlongWait
	out.Money.ExpectedLossCents, out.Money.WorstCaseLossCents = s.Money.ExpectedLoss, s.Money.WorstCase
	out.Money.MRRAtRiskCents, out.Money.TotalMRRCents = s.Money.MRRAtRisk, s.Money.TotalMRR
	out.Money.SavableTop20Cents = s.Money.SavableTop20
	out.TopAtRisk = []customerListItemJSON{}
	for _, id := range s.TopAtRisk {
		if cr, ok := snap.Customer(id); ok {
			out.TopAtRisk = append(out.TopAtRisk, toListItem(cr))
		}
	}
	writeJSON(w, http.StatusOK, out)
}
