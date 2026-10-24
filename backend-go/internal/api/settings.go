package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/features"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
)

const maxSettingsBody = 64 << 10 // 64 KiB: a patch of every feature is a few hundred bytes

// modelSettingsJSON is the ModelSettings schema of openapi.yml.
type modelSettingsJSON struct {
	Version     int64                `json:"version"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	HorizonDays int                  `json:"horizonDays"`
	Thresholds  thresholdsJSON       `json:"thresholds"`
	Clock       clockJSON            `json:"clock"`
	Features    []featureSettingJSON `json:"features"`
}

type thresholdsJSON struct {
	Medium int `json:"medium"`
	High   int `json:"high"`
}

type clockJSON struct {
	Mode    string     `json:"mode"`
	FixedAt *time.Time `json:"fixedAt"`
}

type featureSettingJSON struct {
	Key           string  `json:"key"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Scope         string  `json:"scope"`
	Weight        float64 `json:"weight"`
	DefaultWeight float64 `json:"defaultWeight"`
	Min           float64 `json:"min"`
	Max           float64 `json:"max"`
	Enabled       bool    `json:"enabled"`
}

// toSettingsJSON: features come from the registry (in the database they are a map by key),
// so the UI lists every feature of the code, including ones added after the settings were saved.
func toSettingsJSON(r settings.Record) modelSettingsJSON {
	out := modelSettingsJSON{
		Version:     r.Version,
		UpdatedAt:   r.UpdatedAt,
		HorizonDays: r.HorizonDays,
		Thresholds:  thresholdsJSON{Medium: r.Thresholds.Medium, High: r.Thresholds.High},
		Clock:       clockJSON{Mode: r.Clock.Mode, FixedAt: r.Clock.FixedAt},
	}
	for _, f := range features.All() {
		m := f.Meta()
		fs, ok := r.Features[m.Key]
		if !ok {
			fs = engine.FeatureSetting{Weight: m.DefaultWeight, Enabled: true}
		}
		out.Features = append(out.Features, featureSettingJSON{
			Key: m.Key, Title: m.Title, Description: m.Description, Scope: string(m.Scope),
			Weight: fs.Weight, DefaultWeight: m.DefaultWeight, Min: m.Min, Max: m.Max, Enabled: fs.Enabled,
		})
	}
	return out
}

// getSettings: GET /api/v1/model/settings.
func (h *handlers) getSettings(w http.ResponseWriter, r *http.Request) {
	rec, err := h.settings.Get(r.Context())
	if err != nil {
		writeInternal(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toSettingsJSON(rec))
}

// patchSettings: PATCH /api/v1/model/settings — only the changed fields.
func (h *handlers) patchSettings(w http.ResponseWriter, r *http.Request) {
	p, ok := decodePatch(w, r)
	if !ok {
		return
	}
	rec, err := h.settings.Patch(r.Context(), p)
	h.writeSettingsResult(w, r, rec, err)
}

// resetSettings: POST /api/v1/model/settings/reset.
func (h *handlers) resetSettings(w http.ResponseWriter, r *http.Request) {
	rec, err := h.settings.Reset(r.Context())
	h.writeSettingsResult(w, r, rec, err)
}

func (h *handlers) writeSettingsResult(w http.ResponseWriter, r *http.Request, rec settings.Record, err error) {
	var verr *settings.ValidationError
	switch {
	case errors.As(err, &verr):
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Настройки не прошли проверку", verr.Fields)
	case err != nil:
		writeInternal(w, r, h.logger, err)
	default:
		h.logger.InfoContext(r.Context(), "model settings changed", "version", rec.Version)
		h.refresh(r)
		writeJSON(w, http.StatusOK, toSettingsJSON(rec))
	}
}

// refreshTimeout: the settings response waits for the recalculation at most this long
// (docs/ТЗ_BACKEND_GO.md §10), so the next GET already shows the new numbers.
const refreshTimeout = 2 * time.Second

func (h *handlers) refresh(r *http.Request) {
	if h.recalc == nil {
		return
	}
	if _, err := h.recalc.Refresh(r.Context(), recalc.ReasonSettings, refreshTimeout); err != nil {
		// The settings are saved; the new numbers will arrive with the next calculation.
		h.logger.WarnContext(r.Context(), "settings saved, recalculation still pending", "error", err)
	}
}

// decodePatch reads a ModelSettingsPatch. Unknown fields are rejected: a typo like "horizon"
// would otherwise be silently ignored and the user would think the change was saved.
func decodePatch(w http.ResponseWriter, r *http.Request) (engine.Patch, bool) {
	var p engine.Patch
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSettingsBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Ожидается JSON по схеме ModelSettingsPatch",
			map[string][]string{"body": {err.Error()}})
		return engine.Patch{}, false
	}
	return p, true
}
