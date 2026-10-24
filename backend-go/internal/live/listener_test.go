package live

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/testdb"
)

// Integration tests on a throwaway database; skipped without TEST_DATABASE_URL.

func startListener(t *testing.T, pool *pgxpool.Pool) (*atomic.Int64, *atomic.Int64) {
	t.Helper()
	var changes, reconnects atomic.Int64
	l := &Listener{
		URL:         testdb.URL(t, pool),
		Logger:      slog.New(slog.NewJSONHandler(io.Discard, nil)),
		OnChange:    func() { changes.Add(1) },
		OnReconnect: func() { reconnects.Add(1) },
		MinBackoff:  20 * time.Millisecond,
		MaxBackoff:  50 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return &changes, &reconnects
}

// waitListening waits until the listener session shows up in pg_stat_activity.
func waitListening(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int
		_ = pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND query = 'LISTEN data_changed'`).Scan(&n)
		return n == 1
	}, 5*time.Second, 20*time.Millisecond)
}

func TestNotificationsReachTheCallback(t *testing.T) {
	pool := testdb.New(t)
	changes, _ := startListener(t, pool)
	waitListening(t, pool)

	_, err := pool.Exec(context.Background(), `SELECT pg_notify('data_changed', '')`)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return changes.Load() == 1 }, 2*time.Second, 10*time.Millisecond)

	_, err = pool.Exec(context.Background(), `SELECT pg_notify('other_channel', '')`)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 1, changes.Load(), "other channels are ignored")
}

// The ingest triggers: one statement → one notification, whatever the number of rows.
func TestIngestTriggersNotify(t *testing.T) {
	pool := testdb.New(t)
	testdb.SourceSchema(t, pool)
	_, err := pool.Exec(context.Background(), `
		CREATE FUNCTION public.prolongo_notify_changed() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_notify('data_changed', ''); RETURN NULL; END $$;
		CREATE TRIGGER accounts_notify AFTER INSERT OR UPDATE OR DELETE ON public.accounts
		FOR EACH STATEMENT EXECUTE FUNCTION public.prolongo_notify_changed();`)
	require.NoError(t, err)
	testdb.Seed(t, pool)

	changes, _ := startListener(t, pool)
	waitListening(t, pool)
	_, err = pool.Exec(context.Background(), `UPDATE public.accounts SET balance = 0 WHERE id = 1`)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `UPDATE public.accounts SET balance = balance + 1`) // 10 rows
	require.NoError(t, err)
	require.Eventually(t, func() bool { return changes.Load() == 2 }, 2*time.Second, 10*time.Millisecond)
}

func TestReconnectsAfterConnectionLoss(t *testing.T) {
	pool := testdb.New(t)
	changes, reconnects := startListener(t, pool)
	waitListening(t, pool)
	assert.Zero(t, reconnects.Load(), "the first connection is not a reconnect")

	// Kill the listener's session, as a PostgreSQL restart would.
	_, err := pool.Exec(context.Background(),
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND query = 'LISTEN data_changed'`)
	require.NoError(t, err)

	require.Eventually(t, func() bool { return reconnects.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	waitListening(t, pool)
	_, err = pool.Exec(context.Background(), `SELECT pg_notify('data_changed', '')`)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return changes.Load() == 1 }, 2*time.Second, 10*time.Millisecond, "listening again")
}
