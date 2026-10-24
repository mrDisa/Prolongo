package recalc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
)

// fakeLoader serves data.json and counts loads; fail makes the next loads return an error.
type fakeLoader struct {
	ds    *domain.Dataset
	loads atomic.Int64
	fail  atomic.Bool
}

func (f *fakeLoader) LoadDataset(context.Context) (*domain.Dataset, error) {
	f.loads.Add(1)
	if f.fail.Load() {
		return nil, errors.New("database is down")
	}
	return f.ds, nil
}

type fakeSettings struct {
	mu  sync.Mutex
	rec settings.Record
}

func (f *fakeSettings) Get(context.Context) (settings.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rec, nil
}

func (f *fakeSettings) set(mutate func(*engine.Settings)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	mutate(&f.rec.Settings)
	f.rec.Version++
}

func reference(t *testing.T) *domain.Dataset {
	t.Helper()
	f, err := os.Open("../../../data/test-data/data.json")
	require.NoError(t, err)
	defer f.Close()
	ds, err := domain.FromJSON(f)
	require.NoError(t, err)
	return ds
}

var quiet = slog.New(slog.NewJSONHandler(io.Discard, nil))

func newWorker(t *testing.T, cfg Config) (*Worker, *fakeLoader, *fakeSettings) {
	t.Helper()
	loader := &fakeLoader{ds: reference(t)}
	set := &fakeSettings{rec: settings.Record{Version: 1, Settings: engine.DefaultSettings()}}
	return New(cfg, loader, set, quiet), loader, set
}

func fast() Config {
	return Config{Debounce: 30 * time.Millisecond, MaxDelay: 300 * time.Millisecond, Interval: time.Hour}
}

func start(t *testing.T, w *Worker) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
}

func TestStartupCalculation(t *testing.T) {
	w, loader, _ := newWorker(t, fast())
	assert.Nil(t, w.Current(), "nothing before the first calculation")

	require.NoError(t, w.Recalculate(context.Background(), ReasonStartup))
	snap := w.Current()
	require.NotNil(t, snap)
	assert.EqualValues(t, 1, snap.Version)
	assert.Equal(t, ReasonStartup, snap.Reason)
	assert.EqualValues(t, 1, snap.SettingsVersion)
	assert.Same(t, loader.ds, snap.Dataset, "the input is kept for G8.1")
	assert.Equal(t, time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC), snap.Now, "data clock")
	assert.InDelta(t, 703267, snap.Summary.Money.ExpectedLoss, 2, "the reference numbers")
}

func TestBurstOfSignalsGivesOneCalculation(t *testing.T) {
	w, loader, _ := newWorker(t, fast())
	start(t, w)

	for range 50 { // a snapshot upload: one NOTIFY per statement
		w.Trigger(ReasonData)
	}
	snap, err := w.Refresh(context.Background(), ReasonData, time.Second)
	require.NoError(t, err)
	assert.Equal(t, ReasonData, snap.Reason)
	time.Sleep(100 * time.Millisecond) // give a wrongly debounced second run time to happen
	assert.EqualValues(t, 1, loader.loads.Load(), "51 signals, one load")
	assert.EqualValues(t, 1, w.Current().Version)
}

func TestMaxDelayCapsTheWait(t *testing.T) {
	w, _, _ := newWorker(t, fast())
	start(t, w)

	stop := make(chan struct{})
	defer close(stop)
	go func() { // a signal every 10 ms: the quiet period never comes
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				w.Trigger(ReasonData)
			}
		}
	}()
	began := time.Now()
	_, err := w.Refresh(context.Background(), ReasonData, 2*time.Second)
	require.NoError(t, err)
	assert.Less(t, time.Since(began), 600*time.Millisecond, "MaxDelay (300 ms) forces the calculation")
}

func TestSettingsReasonWins(t *testing.T) {
	w, _, set := newWorker(t, fast())
	start(t, w)
	set.set(func(s *engine.Settings) { s.HorizonDays = 7 })

	w.Trigger(ReasonData)
	snap, err := w.Refresh(context.Background(), ReasonSettings, time.Second)
	require.NoError(t, err)
	assert.Equal(t, ReasonSettings, snap.Reason, "the SSE event must say settings changed")
	assert.Equal(t, 7, snap.HorizonDays, "reads the new settings")
	assert.EqualValues(t, 2, snap.SettingsVersion)
	assert.InDelta(t, 89738, snap.Summary.Money.ExpectedLoss, 2)
}

func TestTimerRecalculates(t *testing.T) {
	cfg := fast()
	cfg.Interval = 50 * time.Millisecond
	w, _, _ := newWorker(t, cfg)
	start(t, w)
	require.Eventually(t, func() bool {
		s := w.Current()
		return s != nil && s.Version >= 2 && s.Reason == ReasonTimer
	}, time.Second, 10*time.Millisecond)
}

func TestFailureKeepsPreviousResult(t *testing.T) {
	w, loader, _ := newWorker(t, fast())
	require.NoError(t, w.Recalculate(context.Background(), ReasonStartup))
	good := w.Current()

	loader.fail.Store(true)
	assert.Error(t, w.Recalculate(context.Background(), ReasonData))
	assert.Same(t, good, w.Current(), "the panel keeps the last good numbers")
	assert.EqualValues(t, 1, w.Errors())

	loader.fail.Store(false)
	require.NoError(t, w.Recalculate(context.Background(), ReasonData))
	assert.EqualValues(t, 2, w.Current().Version, "failed runs do not consume versions")
}

func TestRefreshTimesOut(t *testing.T) {
	w, _, _ := newWorker(t, fast()) // Run is not started: nobody will calculate
	_, err := w.Refresh(context.Background(), ReasonSettings, 50*time.Millisecond)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestHooksRunAfterPublish(t *testing.T) {
	w, _, _ := newWorker(t, fast())
	var got []int64
	w.OnPublish(func(_ context.Context, s *Snapshot) {
		assert.Same(t, s, w.Current(), "hooks see the published snapshot")
		got = append(got, s.Version)
	})
	require.NoError(t, w.Recalculate(context.Background(), ReasonStartup))
	require.NoError(t, w.Recalculate(context.Background(), ReasonData))
	assert.Equal(t, []int64{1, 2}, got)
}

func TestTriggerNeverBlocks(t *testing.T) {
	w, _, _ := newWorker(t, fast()) // nobody reads the queue
	done := make(chan struct{})
	go func() {
		for range 10_000 {
			w.Trigger(ReasonData)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Trigger blocked on a full queue")
	}
}

// Readers never see a half-built result: run with -race.
func TestConcurrentReaders(t *testing.T) {
	cfg := fast()
	cfg.Debounce = time.Millisecond
	w, _, _ := newWorker(t, cfg)
	require.NoError(t, w.Recalculate(context.Background(), ReasonStartup))
	start(t, w)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				s := w.Current()
				assert.Equal(t, 10, s.Summary.Customers.Total)
			}
		})
	}
	for range 20 {
		w.Trigger(ReasonData)
		time.Sleep(2 * time.Millisecond)
	}
	wg.Wait()
}
