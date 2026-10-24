package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/live"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
)

// defaultPing keeps idle connections alive through proxies (docs/ОБЩЕЕ_ТЗ.md §7.9).
const defaultPing = 15 * time.Second

// recalculatedJSON is the RecalculatedEvent schema of openapi.yml.
type recalculatedJSON struct {
	CalcVersion  int64     `json:"calcVersion"`
	CalculatedAt time.Time `json:"calculatedAt"`
	Reason       string    `json:"reason"`
}

// RecalculatedEvent turns a published calculation into the SSE event "recalculated".
func RecalculatedEvent(snap *recalc.Snapshot) live.Event {
	data, _ := json.Marshal(recalculatedJSON{CalcVersion: snap.Version, CalculatedAt: snap.CalculatedAt, Reason: snap.Reason})
	return live.Event{Name: "recalculated", Data: data}
}

var pingEvent = live.Event{Name: "ping", Data: []byte("{}")}

// stream: GET /api/v1/stream — Server-Sent Events. The connection stays open: the panel
// learns about every recalculation and reloads its data, no polling needed.
func (h *handlers) stream(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w) // reaches Flush through middleware wrappers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: send each event at once
	w.WriteHeader(http.StatusOK)

	events, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	// The current state first: a client that reconnects is in sync without asking.
	if snap := h.current.Current(); snap != nil {
		if writeEvent(w, rc, RecalculatedEvent(snap)) != nil {
			return
		}
	} else if rc.Flush() != nil {
		return
	}

	ping := time.NewTicker(h.ping)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done(): // the client went away (or the server is shutting down)
			return
		case e, ok := <-events:
			if !ok { // the hub dropped us: we were too slow
				return
			}
			if writeEvent(w, rc, e) != nil {
				return
			}
		case <-ping.C:
			if writeEvent(w, rc, pingEvent) != nil {
				return
			}
		}
	}
}

func writeEvent(w http.ResponseWriter, rc *http.ResponseController, e live.Event) error {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Name, e.Data); err != nil {
		return err
	}
	return rc.Flush()
}
