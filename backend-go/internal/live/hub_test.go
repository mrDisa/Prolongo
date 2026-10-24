package live

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recv(t *testing.T, ch <-chan Event) (Event, bool) {
	t.Helper()
	select {
	case e, ok := <-ch:
		return e, ok
	case <-time.After(time.Second):
		t.Fatal("no event")
		return Event{}, false
	}
}

func TestBroadcastReachesEveryone(t *testing.T) {
	h := NewHub()
	a, unsubA := h.Subscribe()
	b, unsubB := h.Subscribe()
	defer unsubA()
	defer unsubB()
	assert.Equal(t, 2, h.Count())

	h.Broadcast(Event{Name: "recalculated", Data: []byte(`{"calcVersion":2}`)})
	for _, ch := range []<-chan Event{a, b} {
		e, ok := recv(t, ch)
		require.True(t, ok)
		assert.Equal(t, "recalculated", e.Name)
		assert.JSONEq(t, `{"calcVersion":2}`, string(e.Data))
	}
}

func TestUnsubscribe(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe()
	unsub()
	unsub() // twice is fine
	assert.Zero(t, h.Count())
	_, ok := <-ch
	assert.False(t, ok, "the channel is closed")
	h.Broadcast(Event{Name: "x"}) // no panic on a closed channel
}

// A client that does not read is dropped; the others keep receiving.
func TestSlowSubscriberIsDropped(t *testing.T) {
	h := NewHub()
	slow, unsubSlow := h.Subscribe()
	defer unsubSlow()
	fast, unsubFast := h.Subscribe()
	defer unsubFast()

	for i := range SubscriberBuffer + 1 { // one more than fits
		h.Broadcast(Event{Name: "e", Data: []byte{byte('0' + i%10)}})
		_, ok := recv(t, fast) // the fast one reads every event
		require.True(t, ok)
	}
	assert.Equal(t, 1, h.Count(), "the slow client was removed")

	n := 0
	for range slow { // buffered events are still readable, then the channel ends
		n++
	}
	assert.Equal(t, SubscriberBuffer, n)
}

func TestBroadcastNeverBlocks(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe() // nobody reads
	defer unsub()
	done := make(chan struct{})
	go func() {
		for range 1000 {
			h.Broadcast(Event{Name: "e"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked")
	}
}

func TestCloseEndsAllStreams(t *testing.T) {
	h := NewHub()
	a, unsubA := h.Subscribe()
	b, _ := h.Subscribe()
	h.Close()
	for _, ch := range []<-chan Event{a, b} {
		_, ok := <-ch
		assert.False(t, ok)
	}
	assert.Zero(t, h.Count())
	unsubA() // after Close: no double close
}
