package settings_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings/settingstest"
)

func ptr[T any](v T) *T { return &v }

func newService(t *testing.T) (*settings.Service, *settingstest.MemStore) {
	t.Helper()
	mem := &settingstest.MemStore{}
	svc := settings.NewService(mem)
	_, err := svc.Ensure(context.Background())
	require.NoError(t, err)
	return svc, mem
}

func TestEnsureCreatesDefaultsOnce(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	rec, err := svc.Get(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rec.Version)
	assert.Equal(t, engine.DefaultSettings(), rec.Settings)

	_, err = svc.Patch(ctx, engine.Patch{HorizonDays: ptr(14)})
	require.NoError(t, err)
	rec, err = svc.Ensure(ctx) // a restart must not overwrite what was saved
	require.NoError(t, err)
	assert.Equal(t, 14, rec.HorizonDays)
	assert.EqualValues(t, 2, rec.Version)
}

func TestPatchAndReset(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	rec, err := svc.Patch(ctx, engine.Patch{Features: map[string]engine.FeaturePatch{"prolong_wait": {Weight: ptr(50.0)}}})
	require.NoError(t, err)
	assert.EqualValues(t, 2, rec.Version)
	assert.Equal(t, engine.FeatureSetting{Weight: 50, Enabled: true}, rec.Features["prolong_wait"])

	rec, err = svc.Reset(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 3, rec.Version, "reset is a change too")
	assert.Equal(t, engine.DefaultSettings(), rec.Settings)
}

func TestInvalidPatchSavesNothing(t *testing.T) {
	ctx := context.Background()
	svc, mem := newService(t)
	before := mem.Data()

	_, err := svc.Patch(ctx, engine.Patch{HorizonDays: ptr(14), Thresholds: &engine.ThresholdsPatch{Medium: ptr(90)}})
	var verr *settings.ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Equal(t, engine.ValidationErrors{"thresholds.medium": {"Должен быть меньше high (60)"}}, verr.Fields)

	assert.Equal(t, before, mem.Data(), "the valid part (horizon) is not saved either")
	rec, _ := svc.Get(ctx)
	assert.EqualValues(t, 1, rec.Version)
}

// Settings saved before a feature existed: it appears with its default weight;
// a key the code no longer knows stays in the stored JSON.
func TestMergeWithRegistry(t *testing.T) {
	ctx := context.Background()
	mem := &settingstest.MemStore{}
	mem.SetData([]byte(`{"horizonDays":60,"thresholds":{"medium":25,"high":55},"clock":{"mode":"data","fixedAt":null},
		"features":{"prolong_wait":{"weight":40,"enabled":true},"retired_feature":{"weight":3,"enabled":true}}}`))
	svc := settings.NewService(mem)

	rec, err := svc.Ensure(ctx)
	require.NoError(t, err)
	assert.Equal(t, 60, rec.HorizonDays)
	assert.Equal(t, engine.FeatureSetting{Weight: 40, Enabled: true}, rec.Features["prolong_wait"], "stored value wins")
	assert.Equal(t, engine.FeatureSetting{Weight: 30, Enabled: true}, rec.Features["insufficient_funds"], "missing → default")

	_, err = svc.Patch(ctx, engine.Patch{HorizonDays: ptr(30)})
	require.NoError(t, err)
	var doc struct {
		Features map[string]json.RawMessage `json:"features"`
	}
	require.NoError(t, json.Unmarshal(mem.Data(), &doc))
	assert.Contains(t, doc.Features, "retired_feature", "unknown keys are never deleted")
	assert.Len(t, doc.Features, 10, "9 registered + 1 retired")
}

func TestConcurrentPatchesAreNotLost(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	const n = 20
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			// Each goroutine sets a different horizon; all n changes must be counted.
			_, err := svc.Patch(ctx, engine.Patch{HorizonDays: ptr(10 + i)})
			assert.NoError(t, err)
		})
	}
	wg.Wait()
	rec, err := svc.Get(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1+n, rec.Version)
}
