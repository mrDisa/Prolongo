package api

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
)

// horizonCacheSize: the period switcher has 7 / 14 / 30 / 60 / 90 + "custom" — 8 is plenty.
const horizonCacheSize = 8

var errHorizon = errors.New("horizonDays: целое число от 1 до 365")

// horizonCache keeps results for other horizons of ONE calculation. A new calcVersion
// (data or settings changed) makes them stale, so the cache starts over.
type horizonCache struct {
	mu      sync.Mutex
	version int64
	entries []horizonEntry // most recently used first
}

type horizonEntry struct {
	days   int
	result *engine.Result
}

// get returns the cached result for days, or nil.
func (c *horizonCache) get(version int64, days int) *engine.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.version != version {
		c.version, c.entries = version, nil
		return nil
	}
	for i, e := range c.entries {
		if e.days == days {
			copy(c.entries[1:i+1], c.entries[:i]) // move to the front
			c.entries[0] = e
			return e.result
		}
	}
	return nil
}

// put stores a result, evicting the least recently used one when full.
func (c *horizonCache) put(version int64, days int, r *engine.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.version != version {
		if version < c.version {
			return // a slow request finished after a newer calculation: do not go back
		}
		c.version, c.entries = version, nil
	}
	for _, e := range c.entries {
		if e.days == days {
			return // another request computed it meanwhile
		}
	}
	c.entries = append([]horizonEntry{{days, r}}, c.entries...)
	if len(c.entries) > horizonCacheSize {
		c.entries = c.entries[:horizonCacheSize]
	}
}

func (c *horizonCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// parseHorizon reads the optional horizonDays query parameter; 0 means "not given".
func parseHorizon(r *http.Request) (int, error) {
	s := r.URL.Query().Get("horizonDays")
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 365 {
		return 0, errHorizon
	}
	return n, nil
}

// resultFor returns the calculation for the requested horizon (docs/ОБЩЕЕ_ТЗ.md §7.2):
// without horizonDays or with the saved one — the published result; otherwise the same
// data, weights and "now" with another horizon. Settings are not changed.
func (h *handlers) resultFor(snap *recalc.Snapshot, days int) *engine.Result {
	if days == 0 || days == snap.HorizonDays {
		return snap.Result
	}
	if r := h.horizons.get(snap.Version, days); r != nil {
		return r
	}
	s := snap.Settings
	s.HorizonDays = days
	r := engine.Calculate(snap.Dataset, s, snap.Now) // outside the lock: other requests are not blocked
	h.horizons.put(snap.Version, days, r)
	return r
}

// snapshotAndResult combines snapshot() with horizonDays; it writes 400/500 itself.
func (h *handlers) snapshotAndResult(w http.ResponseWriter, r *http.Request) (*recalc.Snapshot, *engine.Result, bool) {
	days, err := parseHorizon(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error(), nil)
		return nil, nil, false
	}
	snap, ok := h.snapshot(w, r)
	if !ok {
		return nil, nil, false
	}
	return snap, h.resultFor(snap, days), true
}
