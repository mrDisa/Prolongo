package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/store"
	"github.com/mrDisa/Prolongo/backend-go/internal/testdb"
)

func TestTimelineFacts(t *testing.T) {
	ctx := context.Background()
	s, pool := setup(t)
	testdb.SourceSchema(t, pool)
	_, err := pool.Exec(ctx, `
		INSERT INTO public.status_events (service_id, account_id, from_status, to_status, occurred_at, source) VALUES
			(1, 1, 'prolong_wait', 'deleted', '2026-10-18T10:00:00Z', 'simulator'),
			(2, 1, 'active',       'deleted', '2026-10-18T23:59:59Z', 'admin'),
			(3, 2, 'prolong_wait', 'deleted', '2026-10-19T00:00:00Z', 'sql'),
			(4, 2, 'active',    'suspended',  '2026-10-18T12:00:00Z', 'simulator'),
			(5, 3, 'prolong_wait', 'deleted', '2026-01-01T12:00:00Z', 'snapshot'),
			(6, 3, 'prolong_wait', 'deleted', '2026-10-20T00:00:00Z', 'simulator')`)
	require.NoError(t, err)
	for _, d := range []store.RiskDaily{
		{Day: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), ServicesAtRisk: 11},
		{Day: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), ServicesAtRisk: 9},
		{Day: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), ServicesAtRisk: 99}, // tomorrow: outside
	} {
		require.NoError(t, s.UpsertRiskDaily(ctx, d))
	}

	from := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	facts, err := s.TimelineFacts(ctx, from, to)
	require.NoError(t, err)

	assert.Equal(t, map[string]int{"2026-10-18": 2, "2026-10-19": 1}, facts.Deletions,
		"only transitions to deleted, by UTC day, inside [from, to)")
	assert.Equal(t, map[int64]bool{1: true, 2: true, 3: true, 5: true, 6: true}, facts.DeletedWithEvent,
		"any day: an old recorded deletion still replaces the estimate")
	assert.Equal(t, map[string]int{"2026-10-17": 11, "2026-10-19": 9}, facts.ServicesAtRisk)

	empty, err := s.TimelineFacts(ctx, to, to)
	require.NoError(t, err)
	assert.Empty(t, empty.Deletions)
	assert.Empty(t, empty.ServicesAtRisk)
	assert.NotNil(t, empty.Deletions, "maps are ready to read")
}
