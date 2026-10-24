package engine

import (
	"fmt"
	"maps"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/features"
)

// Patch is a partial change of the settings: the body of PATCH /model/settings and of
// POST /model/preview (openapi.yml, ModelSettingsPatch). Pointers tell "not sent" (nil)
// from "sent as zero".
type Patch struct {
	HorizonDays *int                    `json:"horizonDays"`
	Thresholds  *ThresholdsPatch        `json:"thresholds"`
	Clock       *ClockPatch             `json:"clock"`
	Features    map[string]FeaturePatch `json:"features"`
}

// ThresholdsPatch changes one or both thresholds.
type ThresholdsPatch struct {
	Medium *int `json:"medium"`
	High   *int `json:"high"`
}

// ClockPatch replaces the clock as a whole: mode is required, fixedAt only for "fixed".
type ClockPatch struct {
	Mode    string     `json:"mode"`
	FixedAt *time.Time `json:"fixedAt"`
}

// FeaturePatch changes the weight and/or the switch of one feature.
type FeaturePatch struct {
	Weight  *float64 `json:"weight"`
	Enabled *bool    `json:"enabled"`
}

// ValidationErrors maps a field path ("thresholds.medium", "features.prolong_wait.weight")
// to messages for the user. Empty means valid.
type ValidationErrors map[string][]string

func (v ValidationErrors) add(path, msg string) { v[path] = append(v[path], msg) }

// Apply returns s with the patch on top. s itself is not modified: the Features map is
// copied first, because maps are references and the current settings may be in use.
// Unknown feature keys in the patch are reported by Validate via the returned errors.
func Apply(s Settings, p Patch) (Settings, ValidationErrors) {
	errs := ValidationErrors{}
	out := s
	out.Features = maps.Clone(s.Features)
	if out.Features == nil {
		out.Features = map[string]FeatureSetting{}
	}

	if p.HorizonDays != nil {
		out.HorizonDays = *p.HorizonDays
	}
	if p.Thresholds != nil {
		if p.Thresholds.Medium != nil {
			out.Thresholds.Medium = *p.Thresholds.Medium
		}
		if p.Thresholds.High != nil {
			out.Thresholds.High = *p.Thresholds.High
		}
	}
	if p.Clock != nil {
		out.Clock = Clock{Mode: p.Clock.Mode, FixedAt: p.Clock.FixedAt}
		if p.Clock.Mode != ClockFixed {
			out.Clock.FixedAt = nil // fixedAt only means something in fixed mode
		}
	}
	for key, fp := range p.Features {
		f, ok := features.ByKey(key)
		if !ok {
			errs.add("features."+key, "Неизвестный признак")
			continue
		}
		cur := out.Feature(f.Meta())
		if fp.Weight != nil {
			cur.Weight = *fp.Weight
		}
		if fp.Enabled != nil {
			cur.Enabled = *fp.Enabled
		}
		out.Features[key] = cur
	}

	maps.Copy(errs, out.Validate())
	return out, errs
}

// Validate checks the whole settings (docs/ОБЩЕЕ_ТЗ.md §7.8): horizon 1..365,
// 0 < medium < high ≤ 100, known clock mode with fixedAt for "fixed", weights within [min, max].
func (s Settings) Validate() ValidationErrors {
	errs := ValidationErrors{}
	if s.HorizonDays < 1 || s.HorizonDays > 365 {
		errs.add("horizonDays", "Допустимо от 1 до 365 дней")
	}
	switch m, h := s.Thresholds.Medium, s.Thresholds.High; {
	case m < 1:
		errs.add("thresholds.medium", "Должен быть больше 0")
	case h > 100:
		errs.add("thresholds.high", "Должен быть не больше 100")
	case m >= h:
		errs.add("thresholds.medium", fmt.Sprintf("Должен быть меньше high (%d)", h))
	}
	switch s.Clock.Mode {
	case ClockData, ClockReal:
	case ClockFixed:
		if s.Clock.FixedAt == nil {
			errs.add("clock.fixedAt", "Обязательно для режима fixed")
		}
	default:
		errs.add("clock.mode", "Допустимые значения: data, real, fixed")
	}
	for _, f := range features.All() {
		m := f.Meta()
		if w := s.Feature(m).Weight; w < m.Min || w > m.Max {
			errs.add("features."+m.Key+".weight", fmt.Sprintf("Допустимо от %g до %g", m.Min, m.Max))
		}
	}
	return errs
}

// Reset returns the default settings, keeping settings of features that are not in the
// registry any more (a feature removed from the code may come back; its weight is not lost).
func Reset(s Settings) Settings {
	out := DefaultSettings()
	for key, fs := range s.Features {
		if _, known := features.ByKey(key); !known {
			out.Features[key] = fs
		}
	}
	return out
}
