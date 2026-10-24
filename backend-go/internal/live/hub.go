package live

import "sync"

// SubscriberBuffer is how many events may wait for one SSE client (docs/ТЗ_BACKEND_GO.md §12.3).
const SubscriberBuffer = 16

// Event is one Server-Sent Event: "event: <Name>\ndata: <Data>\n\n".
type Event struct {
	Name string
	Data []byte // JSON
}

// Hub delivers events to every connected SSE client.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewHub returns a hub without subscribers.
func NewHub() *Hub {
	return &Hub{subs: map[chan Event]struct{}{}}
}

// Subscribe registers a client. Read events from the channel; when it is closed the hub
// dropped the client (it was too slow) and the handler should end the response.
// Call unsubscribe when the client goes away; calling it twice is fine.
func (h *Hub) Subscribe() (events <-chan Event, unsubscribe func()) {
	ch := make(chan Event, SubscriberBuffer)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() { h.remove(ch) }
}

// Broadcast sends an event to everyone without ever blocking: a client whose buffer is full
// is disconnected instead of slowing down the recalculation goroutine. The browser's
// EventSource reconnects by itself and gets the current state as its first event.
func (h *Hub) Broadcast(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// Close ends every stream: the handlers see their channel closed and return. Called when the
// server shuts down — otherwise open SSE connections would hold graceful shutdown until its timeout.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		delete(h.subs, ch)
		close(ch)
	}
}

// Count reports connected clients (for /metrics).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

func (h *Hub) remove(ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok { // already removed by Broadcast: do not close twice
		delete(h.subs, ch)
		close(ch)
	}
}
