// Package recalc owns the single goroutine that recalculates the risk model and
// publishes the result for everyone else to read.
package recalc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
)

// Reasons of a recalculation; they go to the SSE event "recalculated" (G9).
const (
	ReasonStartup  = "startup"
	ReasonData     = "data"
	ReasonSettings = "settings"
	ReasonTimer    = "timer"
)

// Snapshot is one published calculation. Like engine.Result it is never modified,
// so HTTP handlers read it without locks.
type Snapshot struct {
	*engine.Result
	Version      int64 // calcVersion: grows with every calculation
	CalculatedAt time.Time
	Reason       string
	Duration     time.Duration

	// Dataset and settings the result was built from: the API (G8.1) recalculates other
	// horizons and previews drafts on the same input, without touching the database.
	Dataset         *domain.Dataset
	SettingsVersion int64
}

// Loader reads the source data (*store.Store).
type Loader interface {
	LoadDataset(ctx context.Context) (*domain.Dataset, error)
}

// SettingsSource reads the current model settings (*settings.Service).
type SettingsSource interface {
	Get(ctx context.Context) (settings.Record, error)
}

// Config holds the timings (RECALC_INTERVAL_SECONDS, RECALC_DEBOUNCE_MS).
type Config struct {
	Debounce time.Duration // quiet time after the last signal before recalculating
	MaxDelay time.Duration // never wait longer than this after the first signal, even if signals keep coming
	Interval time.Duration // periodic recalculation: "now" moves even when data does not
}

// Worker recalculates on signals and keeps the latest Snapshot.
type Worker struct {
	cfg      Config
	loader   Loader
	settings SettingsSource
	logger   *slog.Logger
	clock    func() time.Time

	trigger chan string
	current atomic.Pointer[Snapshot]
	version atomic.Int64
	errors  atomic.Int64

	mu        sync.Mutex
	published chan struct{} // closed and replaced on every publish: wakes up Refresh callers
	hooks     []func(context.Context, *Snapshot)
}

// New builds a worker; call Recalculate once at startup, then Run in a goroutine.
func New(cfg Config, loader Loader, settings SettingsSource, logger *slog.Logger) *Worker {
	return &Worker{
		cfg: cfg, loader: loader, settings: settings, logger: logger, clock: time.Now,
		trigger:   make(chan string, 64),
		published: make(chan struct{}),
	}
}

// OnPublish registers a function called after each successful calculation
// (risk_daily now; SSE in G9). Register hooks before Run starts.
func (w *Worker) OnPublish(fn func(context.Context, *Snapshot)) {
	w.hooks = append(w.hooks, fn)
}

// Current returns the latest calculation, or nil before the first one.
func (w *Worker) Current() *Snapshot {
	return w.current.Load()
}

// Errors reports how many calculations failed (for /metrics in G10).
func (w *Worker) Errors() int64 {
	return w.errors.Load()
}

// Trigger asks for a recalculation. It never blocks: if the queue is full, a
// recalculation is already pending and will see the latest data anyway.
func (w *Worker) Trigger(reason string) {
	select {
	case w.trigger <- reason:
	default:
	}
}

// Refresh triggers a recalculation and waits until a newer Snapshot is published or the
// timeout passes. PATCH /model/settings uses it so the response comes after the new numbers exist.
func (w *Worker) Refresh(ctx context.Context, reason string, timeout time.Duration) (*Snapshot, error) {
	before := int64(0)
	if cur := w.Current(); cur != nil {
		before = cur.Version
	}
	w.Trigger(reason)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		w.mu.Lock()
		wait := w.published
		w.mu.Unlock()
		if cur := w.Current(); cur != nil && cur.Version > before {
			return cur, nil
		}
		select {
		case <-wait:
		case <-ctx.Done():
			return w.Current(), fmt.Errorf("recalc: no new result within %s: %w", timeout, ctx.Err())
		}
	}
}

// Run is the only goroutine that calculates. It waits for signals, merges bursts
// (debounce) and also recalculates on a timer. It returns when ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
		var reason string
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reason = ReasonTimer
		case first := <-w.trigger:
			reason = w.debounce(ctx, first)
			if ctx.Err() != nil {
				return
			}
		}
		_ = w.Recalculate(ctx, reason) // errors are logged and counted inside
	}
}

// debounce waits until signals stop for cfg.Debounce, but no longer than cfg.MaxDelay
// after the first one. A snapshot upload fires dozens of NOTIFYs; we calculate once.
// If a settings change is among the signals, the reason is "settings".
func (w *Worker) debounce(ctx context.Context, first string) string {
	reason := first
	deadline := time.After(w.cfg.MaxDelay)
	quiet := time.NewTimer(w.cfg.Debounce)
	defer quiet.Stop()
	for {
		select {
		case <-ctx.Done():
			return reason
		case <-deadline:
			return reason
		case <-quiet.C:
			return reason
		case next := <-w.trigger:
			if next == ReasonSettings {
				reason = ReasonSettings
			}
			quiet.Reset(w.cfg.Debounce)
		}
	}
}

// Recalculate loads the data and settings, calculates and publishes. On error the previous
// Snapshot stays in place, so the panel keeps showing the last good numbers.
func (w *Worker) Recalculate(ctx context.Context, reason string) error {
	start := w.clock()
	snap, err := w.calculate(ctx, reason, start)
	if err != nil {
		w.errors.Add(1)
		w.logger.Error("recalculation failed, keeping the previous result", slog.String("reason", reason), slog.Any("error", err))
		return err
	}
	w.current.Store(snap)
	w.mu.Lock()
	close(w.published)
	w.published = make(chan struct{})
	w.mu.Unlock()

	s := snap.Summary
	w.logger.Info("recalculated",
		slog.Int64("calc_version", snap.Version), slog.String("reason", reason),
		slog.Float64("duration_ms", float64(snap.Duration.Microseconds())/1000),
		slog.Time("model_now", snap.Now), slog.Int("customers", s.Customers.Total),
		slog.Int("high", s.Customers.High), slog.Int("medium", s.Customers.Medium), slog.Int("low", s.Customers.Low),
		slog.Int64("expected_loss_cents", s.Money.ExpectedLoss))

	for _, hook := range w.hooks {
		hook(ctx, snap)
	}
	return nil
}

func (w *Worker) calculate(ctx context.Context, reason string, start time.Time) (*Snapshot, error) {
	ds, err := w.loader.LoadDataset(ctx)
	if err != nil {
		return nil, fmt.Errorf("recalc: %w", err)
	}
	rec, err := w.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("recalc: %w", err)
	}
	if ds == nil {
		return nil, errors.New("recalc: loader returned no dataset")
	}
	now := engine.Now(rec.Clock, ds, w.clock())
	result := engine.Calculate(ds, rec.Settings, now)
	finished := w.clock()
	return &Snapshot{
		Result:          result,
		Version:         w.version.Add(1),
		CalculatedAt:    finished.UTC(),
		Reason:          reason,
		Duration:        finished.Sub(start),
		Dataset:         ds,
		SettingsVersion: rec.Version,
	}, nil
}
