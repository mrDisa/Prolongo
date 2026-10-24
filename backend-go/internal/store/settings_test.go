package store_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"

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
