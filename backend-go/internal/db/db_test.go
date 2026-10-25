package db

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/testdb"
)

// Integration tests run on a throwaway database (internal/testdb) and are
// skipped without TEST_DATABASE_URL.

var quiet = slog.New(slog.NewJSONHandler(io.Discard, nil))

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)

	require.NoError(t, Migrate(ctx, pool, quiet))
	require.NoError(t, Migrate(ctx, pool, quiet), "second run must be a no-op")

	for _, table := range []string{"core.users", "core.sessions", "core.model_settings", "core.model_settings_history", "core.risk_daily", "core.goose_db_version"} {
		var name *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, table).Scan(&name))
		assert.NotNil(t, name, table)
	}
	var version int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT max(version_id) FROM core.goose_db_version`).Scan(&version))
	assert.EqualValues(t, 2, version)
}

func TestWaitForSource(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)

	err := WaitForSource(ctx, pool, 200*time.Millisecond, quiet)
	assert.ErrorContains(t, err, "source schema did not appear", "empty database: no public.services")

	testdb.SourceSchema(t, pool)
	assert.NoError(t, WaitForSource(ctx, pool, time.Second, quiet))
}

func TestOpenFailsFastOnBadURL(t *testing.T) {
	_, err := Open(context.Background(), "postgres://%zz", quiet)
	assert.ErrorContains(t, err, "DATABASE_URL")
}
