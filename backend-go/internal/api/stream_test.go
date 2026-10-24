package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/live"
)

// sseClient reads events from a real HTTP connection: httptest.NewRecorder buffers the
// whole response, so it cannot show that events arrive one by one.
type sseClient struct {
	resp   *http.Response
	lines  *bufio.Scanner
	cancel context.CancelFunc
}

type sseEvent struct{ name, data string }

func openStream(t *testing.T, srv *httptest.Server) *sseClient {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err := client.Post(srv.URL+"/api/v1/auth/login", "application/json",
		strings.NewReader(`{"username":"admin","password":"admin123"}`))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/stream", nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	c := &sseClient{resp: resp, lines: bufio.NewScanner(resp.Body), cancel: cancel}
	t.Cleanup(func() { cancel(); resp.Body.Close() })
	return c
}

// next reads one "event: …\ndata: …\n\n" block, or fails after the timeout.
func (c *sseClient) next(t *testing.T) sseEvent {
	t.Helper()
	got := make(chan sseEvent, 1)
	go func() {
		var e sseEvent
		for c.lines.Scan() {
			line := c.lines.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				e.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				e.data = strings.TrimPrefix(line, "data: ")
			case line == "":
				got <- e
				return
			}
		}
		close(got)
	}()
	select {
	case e, ok := <-got:
		require.True(t, ok, "stream ended")
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("no event within 3 s")
		return sseEvent{}
	}
}

func newStreamServer(t *testing.T, ping time.Duration) (*httptest.Server, *live.Hub) {
	t.Helper()
	d := testDeps(t, nil)
	d.Hub = live.NewHub()
	d.Ping = ping
	srv := httptest.NewServer(NewRouter(d))
	t.Cleanup(srv.Close)
	return srv, d.Hub
}

func TestStreamSendsCurrentStateFirst(t *testing.T) {
	srv, _ := newStreamServer(t, time.Hour)
	c := openStream(t, srv)

	assert.Equal(t, http.StatusOK, c.resp.StatusCode)
	assert.Equal(t, "text/event-stream", c.resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", c.resp.Header.Get("Cache-Control"))
	assert.Equal(t, "no", c.resp.Header.Get("X-Accel-Buffering"))

	e := c.next(t)
	assert.Equal(t, "recalculated", e.name)
	assert.JSONEq(t, `{"calcVersion":17,"calculatedAt":"2026-10-19T14:35:54.204Z","reason":"startup"}`, e.data)
}

func TestStreamDeliversBroadcasts(t *testing.T) {
	srv, hub := newStreamServer(t, time.Hour)
	c := openStream(t, srv)
	c.next(t) // current state

	hub.Broadcast(live.Event{Name: "recalculated", Data: []byte(`{"calcVersion":18,"calculatedAt":"2026-10-19T14:36:05.1Z","reason":"settings"}`)})
	e := c.next(t)
	assert.Equal(t, "recalculated", e.name)
	assert.Contains(t, e.data, `"reason":"settings"`)
}

func TestStreamPings(t *testing.T) {
	srv, _ := newStreamServer(t, 50*time.Millisecond)
	c := openStream(t, srv)
	c.next(t)
	e := c.next(t)
	assert.Equal(t, sseEvent{name: "ping", data: "{}"}, e)
}

func TestStreamEndsWhenClientLeaves(t *testing.T) {
	srv, hub := newStreamServer(t, time.Hour)
	c := openStream(t, srv)
	c.next(t)
	require.Equal(t, 1, hub.Count())

	c.cancel() // the browser tab is closed
	require.Eventually(t, func() bool { return hub.Count() == 0 }, 2*time.Second, 10*time.Millisecond,
		"the handler unsubscribed and returned")
}

func TestStreamEndsWhenHubDropsIt(t *testing.T) {
	srv, hub := newStreamServer(t, time.Hour)
	c := openStream(t, srv)
	c.next(t)

	hub.Close() // what happens on server shutdown or for a too slow client
	done := make(chan struct{})
	go func() {
		for c.lines.Scan() {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the server kept the stream open")
	}
}

func TestRecalculatedEvent(t *testing.T) {
	snap := referenceSnapshot(t).snap
	e := RecalculatedEvent(snap)
	assert.Equal(t, "recalculated", e.Name)
	assert.JSONEq(t, `{"calcVersion":17,"calculatedAt":"2026-10-19T14:35:54.204Z","reason":"startup"}`, string(e.Data))
}
