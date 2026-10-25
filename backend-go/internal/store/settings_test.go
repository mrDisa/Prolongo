package store_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

func TestSettingsRow(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()

	_, err := s.SettingsRow(ctx)
	assert.ErrorIs(t, err, store.ErrNotFound)

	row, err := s.InsertSettings(ctx, []byte(`{"n":1}`))
	require.NoError(t, err)
	assert.EqualValues(t, 1, row.Version)
	assert.JSONEq(t, `{"n":1}`, string(row.Data))

	again, err := s.InsertSettings(ctx, []byte(`{"n":999}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"n":1}`, string(again.Data), "an existing row is never overwritten")

	next, err := s.UpdateSettings(ctx, func(cur store.SettingsRow) ([]byte, error) {
		assert.EqualValues(t, 1, cur.Version)
		return []byte(`{"n":2}`), nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, next.Version)
	assert.True(t, !next.UpdatedAt.Before(row.UpdatedAt))
}

// Every saved version stays in the history, newest first; a failed update adds nothing.
func TestSettingsHistory(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	_, err := s.InsertSettings(ctx, []byte(`{"n":1}`))
	require.NoError(t, err)
	_, err = s.InsertSettings(ctx, []byte(`{"n":999}`))
	require.NoError(t, err)
	for _, data := range []string{`{"n":2}`, `{"n":3}`} {
		_, err := s.UpdateSettings(ctx, func(store.SettingsRow) ([]byte, error) { return []byte(data), nil })
		require.NoError(t, err)
	}
	_, err = s.UpdateSettings(ctx, func(store.SettingsRow) ([]byte, error) { return nil, errors.New("invalid") })
	require.Error(t, err)

	history, err := s.SettingsHistory(ctx, 10)
	require.NoError(t, err)
	require.Len(t, history, 3)
	for i, want := range []string{`{"n":3}`, `{"n":2}`, `{"n":1}`} {
		assert.EqualValues(t, 3-i, history[i].Version)
		assert.JSONEq(t, want, string(history[i].Data))
	}
	cur, err := s.SettingsRow(ctx)
	require.NoError(t, err)
	assert.True(t, cur.UpdatedAt.Equal(history[0].UpdatedAt), "the newest version is the current row")

	limited, err := s.SettingsHistory(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestUpdateSettingsRollsBackOnError(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	_, err := s.InsertSettings(ctx, []byte(`{"n":1}`))
	require.NoError(t, err)

	boom := errors.New("invalid")
	_, err = s.UpdateSettings(ctx, func(store.SettingsRow) ([]byte, error) { return nil, boom })
	assert.ErrorIs(t, err, boom, "the caller's error comes back unchanged")

	row, err := s.SettingsRow(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, row.Version)
	assert.JSONEq(t, `{"n":1}`, string(row.Data))
}

// SELECT … FOR UPDATE: concurrent read-modify-write updates wait for each other,
// so every increment survives. Without the lock some would read the same value and be lost.
func TestUpdateSettingsLocksTheRow(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	_, err := s.InsertSettings(ctx, []byte(`{"n":0}`))
	require.NoError(t, err)

	const workers = 10
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			_, err := s.UpdateSettings(ctx, func(cur store.SettingsRow) ([]byte, error) {
				var n int
				_, scanErr := fmt.Sscanf(string(cur.Data), `{"n": %d}`, &n)
				if scanErr != nil {
					return nil, scanErr
				}
				return []byte(`{"n":` + strconv.Itoa(n+1) + `}`), nil
			})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	row, err := s.SettingsRow(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1+workers, row.Version)
	assert.JSONEq(t, `{"n":10}`, string(row.Data), "no lost update")
}

func TestUpsertRiskDaily(t *testing.T) {
	s, pool := setup(t)
	ctx := context.Background()
	day := time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC)

	require.NoError(t, s.UpsertRiskDaily(ctx, store.RiskDaily{Day: day, CustomersHigh: 2, ServicesAtRisk: 10, ExpectedLossCents: 703267}))
	require.NoError(t, s.UpsertRiskDaily(ctx, store.RiskDaily{Day: day.Add(3 * time.Hour), CustomersHigh: 3, ServicesAtRisk: 11, ExpectedLossCents: 800000}))
	require.NoError(t, s.UpsertRiskDaily(ctx, store.RiskDaily{Day: day.AddDate(0, 0, 1), CustomersHigh: 1}))

	var rows int
	var high, atRisk int
	var loss int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM core.risk_daily`).Scan(&rows))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT customers_high, services_at_risk, expected_loss_cents FROM core.risk_daily WHERE day = '2026-10-19'`).Scan(&high, &atRisk, &loss))
	assert.Equal(t, 2, rows, "one row per day")
	assert.Equal(t, [3]int64{3, 11, 800000}, [3]int64{int64(high), int64(atRisk), loss}, "the latest calculation of the day wins")
}
