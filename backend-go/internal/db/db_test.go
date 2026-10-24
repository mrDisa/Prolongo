package db

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests: they need a disposable PostgreSQL in TEST_DATABASE_URL
// (CI starts one; locally see backend-go/README.md). Without it they are skipped.
func testURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}

var quiet = slog.New(slog.NewJSONHandler(io.Discard, nil))

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testURL(t), quiet)
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, Migrate(ctx, pool, quiet))
	require.NoError(t, Migrate(ctx, pool, quiet), "second run must be a no-op")

	for _, table := range []string{"core.users", "core.sessions", "core.model_settings", "core.risk_daily", "core.goose_db_version"} {
		var name *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, table).Scan(&name))
		assert.NotNil(t, name, table)
	}
	var version int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT max(version_id) FROM core.goose_db_version`).Scan(&version))
	assert.EqualValues(t, 1, version)
}

func TestWaitForSource(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testURL(t), quiet)
	require.NoError(t, err)
	defer pool.Close()

	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.services') IS NOT NULL`).Scan(&exists))
	if !exists {
		err := WaitForSource(ctx, pool, 200*time.Millisecond, quiet)
		assert.ErrorContains(t, err, "source schema did not appear")
		// Stand-in for the table Django creates; never dropped, so it is safe on a shared dev database.
		_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.services (id bigint PRIMARY KEY)`)
		require.NoError(t, err)
	}
	assert.NoError(t, WaitForSource(ctx, pool, time.Second, quiet))
}

func TestOpenFailsFastOnBadURL(t *testing.T) {
	_, err := Open(context.Background(), "postgres://%zz", quiet)
	assert.ErrorContains(t, err, "DATABASE_URL")
}
