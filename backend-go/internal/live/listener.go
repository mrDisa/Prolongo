// Package live turns database changes into recalculation signals (and, in G9, into SSE).
package live

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// Channel is the NOTIFY channel of the ingest triggers (ingest-django/source/sql/triggers.up.sql).
const Channel = "data_changed"

// Listener keeps a dedicated connection with LISTEN data_changed open.
// A pool connection cannot be used: LISTEN belongs to one session and the pool
// may hand that session to someone else or close it.
type Listener struct {
	URL    string
	Logger *slog.Logger
	// OnChange is called for every notification (the trigger fires once per statement).
	OnChange func()
	// OnReconnect is called after the connection is re-established: notifications sent
	// while we were offline are lost, so the caller should recalculate anyway.
	OnReconnect func()

	MinBackoff, MaxBackoff time.Duration // 1 s → 10 s by default
}

// Run listens until ctx is cancelled, reconnecting with exponential backoff.
func (l *Listener) Run(ctx context.Context) {
	minB, maxB := l.MinBackoff, l.MaxBackoff
	if minB <= 0 {
		minB = time.Second
	}
	if maxB <= 0 {
		maxB = 10 * time.Second
	}
	backoff := minB
	for first := true; ; first = false {
		err := l.listen(ctx, func() {
			backoff = minB // connected: next failure starts from the short pause again
			if !first && l.OnReconnect != nil {
				l.OnReconnect()
			}
		})
		if ctx.Err() != nil {
			return
		}
		l.Logger.Warn("LISTEN connection lost, reconnecting", slog.Any("error", err), slog.Duration("in", backoff))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxB)
	}
}

// listen opens one connection and waits for notifications until an error.
func (l *Listener) listen(ctx context.Context, connected func()) error {
	conn, err := pgx.Connect(ctx, l.URL)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, "LISTEN "+Channel); err != nil {
		return err
	}
	l.Logger.Info("listening for data changes", slog.String("channel", Channel))
	connected()

	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return err
		}
		if l.OnChange != nil {
			l.OnChange()
		}
	}
}
