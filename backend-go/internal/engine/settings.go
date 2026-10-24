package engine

import (
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

// Clock modes: where "now" comes from (docs/ОБЩЕЕ_ТЗ.md §4.1).
const (
	ClockData  = "data"  // source time (public.source_clock), the default
	ClockReal  = "real"  // system time
	ClockFixed = "fixed" // a fixed moment, Clock.FixedAt
)

// Settings are the tunable parameters of the model. G6 stores them in core.model_settings.
type Settings struct {
	HorizonDays int
	Thresholds  Thresholds
	Clock       Clock
	// Features overrides per feature key. A feature missing here uses its Meta defaults,
	// so a newly added feature works without touching saved settings.
	Features map[string]FeatureSetting
}

// Thresholds split scores into levels: score ≥ High → high, ≥ Medium → medium, otherwise low.
type Thresholds struct {
	Medium int
	High   int
}

// Clock selects "now" for the calculation.
type Clock struct {
	Mode    string
	FixedAt *time.Time
}

// FeatureSetting is the weight and on/off switch of one feature.
type FeatureSetting struct {
	Weight  float64
	Enabled bool
}

// DefaultSettings: horizon 30 days, thresholds 30 / 60, data clock, every feature at its default weight.
func DefaultSettings() Settings {
	s := Settings{
		HorizonDays: 30,
		Thresholds:  Thresholds{Medium: 30, High: 60},
		Clock:       Clock{Mode: ClockData},
		Features:    map[string]FeatureSetting{},
	}
	for _, f := range features.All() {
		s.Features[f.Meta().Key] = FeatureSetting{Weight: f.Meta().DefaultWeight, Enabled: true}
	}
	return s
}

// Feature returns the effective setting of a feature: the override or the Meta default.
func (s Settings) Feature(m features.Meta) FeatureSetting {
	if fs, ok := s.Features[m.Key]; ok {
		return fs
	}
	return FeatureSetting{Weight: m.DefaultWeight, Enabled: true}
}

// level maps a rounded score to a risk level.
func (s Settings) level(score int) string {
	switch {
	case score >= s.Thresholds.High:
		return LevelHigh
	case score >= s.Thresholds.Medium:
		return LevelMedium
	default:
		return LevelLow
	}
}
