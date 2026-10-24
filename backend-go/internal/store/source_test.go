package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
	"github.com/mrDisa/Prolongo/backend-go/internal/testdb"
)

func referenceJSON(t *testing.T) *domain.Dataset {
	t.Helper()
	f, err := os.Open("../../../data/test-data/data.json")
	require.NoError(t, err)
	defer f.Close()
	ds, err := domain.FromJSON(f)
	require.NoError(t, err)
	return ds
}

// The engine tests (G5) run on data.json without a database. This test proves
// that reading the same data from PostgreSQL gives exactly the same Dataset.
func TestLoadDatasetMatchesDataJSON(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.SourceSchema(t, pool)
	testdb.Seed(t, pool)

	got, err := store.New(pool).LoadDataset(ctx)
	require.NoError(t, err)

	a, s, tx := got.Counts()
	assert.Equal(t, [3]int{10, 30, 100}, [3]int{a, s, tx})
	assert.Equal(t, time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC), got.LatestTimestamp)
	assert.Nil(t, got.SourceTime, "source_clock has no row yet")
	assert.Equal(t, referenceJSON(t), got)
}

func TestLoadDatasetReadsSourceClock(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.SourceSchema(t, pool)
	testdb.Seed(t, pool)

	clock := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `INSERT INTO public.source_clock (id, data_time) VALUES (1, $1)`, clock)
	require.NoError(t, err)

	got, err := store.New(pool).LoadDataset(ctx)
	require.NoError(t, err)
	require.NotNil(t, got.SourceTime)
	assert.Equal(t, clock, *got.SourceTime)
}

func TestLoadDatasetEmptySource(t *testing.T) {
	pool := testdb.New(t)
	testdb.SourceSchema(t, pool)

	got, err := store.New(pool).LoadDataset(context.Background())
	require.NoError(t, err)
	a, s, tx := got.Counts()
	assert.Zero(t, a+s+tx)
	assert.True(t, got.LatestTimestamp.IsZero())
}

func TestLoadDatasetWithoutSourceTables(t *testing.T) {
	_, err := store.New(testdb.New(t)).LoadDataset(context.Background())
	assert.ErrorContains(t, err, "load accounts", "the error names the step that failed")
}
