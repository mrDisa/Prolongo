package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

func TestApplyPatch(t *testing.T) {
	base := DefaultSettings()
	fixed := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	got, errs := Apply(base, Patch{
		HorizonDays: ptr(14),
		Thresholds:  &ThresholdsPatch{High: ptr(65)},
		Clock:       &ClockPatch{Mode: ClockFixed, FixedAt: &fixed},
		Features:    map[string]FeaturePatch{"prolong_wait": {Weight: ptr(50.0)}, "suspended": {Enabled: ptr(false)}},
	})
	assert.Empty(t, errs)
	assert.Equal(t, 14, got.HorizonDays)
	assert.Equal(t, Thresholds{Medium: 30, High: 65}, got.Thresholds, "medium untouched")
	assert.Equal(t, Clock{Mode: ClockFixed, FixedAt: &fixed}, got.Clock)
	assert.Equal(t, FeatureSetting{Weight: 50, Enabled: true}, got.Features["prolong_wait"])
	assert.Equal(t, FeatureSetting{Weight: 8, Enabled: false}, got.Features["suspended"], "weight kept, switched off")

	// The original settings are untouched: Apply copies the map before writing.
	assert.Equal(t, 30, base.HorizonDays)
	assert.Equal(t, FeatureSetting{Weight: 45, Enabled: true}, base.Features["prolong_wait"])
}

func TestApplyPatchClockDropsFixedAt(t *testing.T) {
	fixed := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	base := DefaultSettings()
	base.Clock = Clock{Mode: ClockFixed, FixedAt: &fixed}
	got, errs := Apply(base, Patch{Clock: &ClockPatch{Mode: ClockReal, FixedAt: &fixed}})
	assert.Empty(t, errs)
	assert.Equal(t, Clock{Mode: ClockReal}, got.Clock, "fixedAt only in fixed mode")
}

func TestApplyPatchErrors(t *testing.T) {
	cases := []struct {
		name  string
		patch Patch
		want  ValidationErrors
	}{
		{"horizon too small", Patch{HorizonDays: ptr(0)}, ValidationErrors{"horizonDays": {"Допустимо от 1 до 365 дней"}}},
		{"horizon too big", Patch{HorizonDays: ptr(366)}, ValidationErrors{"horizonDays": {"Допустимо от 1 до 365 дней"}}},
		{"medium above high", Patch{Thresholds: &ThresholdsPatch{Medium: ptr(70)}},
			ValidationErrors{"thresholds.medium": {"Должен быть меньше high (60)"}}},
		{"medium equals high", Patch{Thresholds: &ThresholdsPatch{Medium: ptr(50), High: ptr(50)}},
			ValidationErrors{"thresholds.medium": {"Должен быть меньше high (50)"}}},
		{"medium zero", Patch{Thresholds: &ThresholdsPatch{Medium: ptr(0)}}, ValidationErrors{"thresholds.medium": {"Должен быть больше 0"}}},
		{"high above 100", Patch{Thresholds: &ThresholdsPatch{High: ptr(101)}}, ValidationErrors{"thresholds.high": {"Должен быть не больше 100"}}},
		{"unknown clock", Patch{Clock: &ClockPatch{Mode: "lunar"}}, ValidationErrors{"clock.mode": {"Допустимые значения: data, real, fixed"}}},
		{"fixed without date", Patch{Clock: &ClockPatch{Mode: ClockFixed}}, ValidationErrors{"clock.fixedAt": {"Обязательно для режима fixed"}}},
		{"weight out of range", Patch{Features: map[string]FeaturePatch{"prolong_wait": {Weight: ptr(150.0)}}},
			ValidationErrors{"features.prolong_wait.weight": {"Допустимо от 0 до 100"}}},
		{"positive weight for loyal_payer", Patch{Features: map[string]FeaturePatch{"loyal_payer": {Weight: ptr(5.0)}}},
			ValidationErrors{"features.loyal_payer.weight": {"Допустимо от -50 до 0"}}},
		{"unknown feature", Patch{Features: map[string]FeaturePatch{"vip": {Weight: ptr(1.0)}}},
			ValidationErrors{"features.vip": {"Неизвестный признак"}}},
		{"several at once", Patch{HorizonDays: ptr(0), Clock: &ClockPatch{Mode: "lunar"}},
			ValidationErrors{"horizonDays": {"Допустимо от 1 до 365 дней"}, "clock.mode": {"Допустимые значения: data, real, fixed"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := Apply(DefaultSettings(), tc.patch)
			assert.Equal(t, tc.want, errs)
		})
	}
}

func TestDefaultSettingsAreValid(t *testing.T) {
	assert.Empty(t, DefaultSettings().Validate())
}

func TestReset(t *testing.T) {
	s := DefaultSettings()
	s.HorizonDays = 90
	s.Features["prolong_wait"] = FeatureSetting{Weight: 99, Enabled: false}
	s.Features["retired_feature"] = FeatureSetting{Weight: 3, Enabled: true}

	r := Reset(s)
	assert.Equal(t, 30, r.HorizonDays)
	assert.Equal(t, FeatureSetting{Weight: 45, Enabled: true}, r.Features["prolong_wait"])
	assert.Equal(t, FeatureSetting{Weight: 3, Enabled: true}, r.Features["retired_feature"], "unknown keys survive")
}
