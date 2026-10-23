package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/config"
)

func TestHealthURL(t *testing.T) {
	cases := map[string]string{
		"":             "http://127.0.0.1:8000/healthz",
		":8000":        "http://127.0.0.1:8000/healthz",
		"0.0.0.0:9000": "http://127.0.0.1:9000/healthz",
		"[::]:9001":    "http://127.0.0.1:9001/healthz",
		"garbage":      "http://127.0.0.1:8000/healthz",
	}
	for addr, want := range cases {
		assert.Equal(t, want, healthURL(addr), "addr %q", addr)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().String()
}

// The server answers /healthz, the -healthcheck probe agrees, and cancelling
// the context stops the server cleanly.
func TestRunServesAndShutsDown(t *testing.T) {
	addr := freeAddr(t)
	cfg := config.Config{HTTPAddr: addr}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, logger) }()

	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 3*time.Second, 20*time.Millisecond)

	assert.Equal(t, 0, runHealthcheck(addr))

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(shutdownTimeout + time.Second):
		t.Fatal("server did not stop")
	}

	assert.Equal(t, 1, runHealthcheck(addr), "probe must fail once the server is down")
}
