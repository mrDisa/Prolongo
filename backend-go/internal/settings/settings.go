// Package settings keeps the model settings in core.model_settings: creates defaults on
// first start, merges them with the feature registry, validates and saves changes.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

// Store is what this package needs from the database; *store.Store implements it,
// tests use settingstest.MemStore.
type Store interface {
	SettingsRow(ctx context.Context) (store.SettingsRow, error)
	InsertSettings(ctx context.Context, data []byte) (store.SettingsRow, error)
	UpdateSettings(ctx context.Context, change func(store.SettingsRow) ([]byte, error)) (store.SettingsRow, error)
	SettingsHistory(ctx context.Context, limit int) ([]store.SettingsRow, error)
}

// Record is the current settings with their version.
type Record struct {
	Version   int64
	UpdatedAt time.Time
	engine.Settings
}

// ValidationError carries field errors for a 422 response.
type ValidationError struct {
	Fields engine.ValidationErrors
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("settings: %d invalid fields", len(e.Fields))
}

// Service reads and changes the settings.
type Service struct {
	store Store
}

// NewService returns a settings service on top of a store.
func NewService(s Store) *Service {
	return &Service{store: s}
}

// Ensure creates the default settings on first start; later starts keep what is stored.
func (s *Service) Ensure(ctx context.Context) (Record, error) {
	row, err := s.store.SettingsRow(ctx)
	if errors.Is(err, store.ErrNotFound) {
		data, encErr := encode(engine.DefaultSettings())
		if encErr != nil {
			return Record{}, encErr
		}
		row, err = s.store.InsertSettings(ctx, data)
	}
	if err != nil {
		return Record{}, fmt.Errorf("settings: %w", err)
	}
	return decodeRow(row)
}

// Get returns the stored settings merged with the registry.
func (s *Service) Get(ctx context.Context) (Record, error) {
	row, err := s.store.SettingsRow(ctx)
	if err != nil {
		return Record{}, fmt.Errorf("settings: %w", err)
	}
	return decodeRow(row)
}

// History returns the saved versions, newest first (the first one is the current settings),
// each merged with the registry like Get.
func (s *Service) History(ctx context.Context, limit int) ([]Record, error) {
	rows, err := s.store.SettingsHistory(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		rec, err := decodeRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// Patch applies a partial change in one transaction. Invalid input returns *ValidationError
// and saves nothing.
func (s *Service) Patch(ctx context.Context, p engine.Patch) (Record, error) {
	return s.update(ctx, func(cur engine.Settings) (engine.Settings, error) {
		next, errs := engine.Apply(cur, p)
		if len(errs) > 0 {
			return engine.Settings{}, &ValidationError{Fields: errs}
		}
		return next, nil
	})
}

// Reset returns weights, switches, thresholds, horizon and clock to the defaults.
func (s *Service) Reset(ctx context.Context) (Record, error) {
	return s.update(ctx, func(cur engine.Settings) (engine.Settings, error) {
		return engine.Reset(cur), nil
	})
}

func (s *Service) update(ctx context.Context, change func(engine.Settings) (engine.Settings, error)) (Record, error) {
	row, err := s.store.UpdateSettings(ctx, func(cur store.SettingsRow) ([]byte, error) {
		settings, err := decode(cur.Data)
		if err != nil {
			return nil, err
		}
		next, err := change(settings)
		if err != nil {
			return nil, err
		}
		return encode(next)
	})
	if err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			return Record{}, verr
		}
		return Record{}, fmt.Errorf("settings: %w", err)
	}
	return decodeRow(row)
}

// document is the JSON stored in core.model_settings.data. Features is a map by key:
// keys of features removed from the code stay in the JSON untouched.
type document struct {
	HorizonDays int `json:"horizonDays"`
	Thresholds  struct {
		Medium int `json:"medium"`
		High   int `json:"high"`
	} `json:"thresholds"`
	Clock struct {
		Mode    string     `json:"mode"`
		FixedAt *time.Time `json:"fixedAt"`
	} `json:"clock"`
	Features map[string]featureDoc `json:"features"`
}

type featureDoc struct {
	Weight  float64 `json:"weight"`
	Enabled bool    `json:"enabled"`
}

func encode(s engine.Settings) ([]byte, error) {
	var d document
	d.HorizonDays = s.HorizonDays
	d.Thresholds.Medium, d.Thresholds.High = s.Thresholds.Medium, s.Thresholds.High
	d.Clock.Mode, d.Clock.FixedAt = s.Clock.Mode, s.Clock.FixedAt
	d.Features = make(map[string]featureDoc, len(s.Features))
	for k, f := range s.Features {
		d.Features[k] = featureDoc{Weight: f.Weight, Enabled: f.Enabled}
	}
	data, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("settings: encode: %w", err)
	}
	return data, nil
}

// decode reads the stored JSON and merges it with the registry: a feature that exists in the
// code but not in the JSON gets its default weight; unknown keys are kept as they are.
func decode(data []byte) (engine.Settings, error) {
	var d document
	if err := json.Unmarshal(data, &d); err != nil {
		return engine.Settings{}, fmt.Errorf("settings: stored JSON: %w", err)
	}
	s := engine.DefaultSettings() // registry defaults first…
	s.HorizonDays = d.HorizonDays
	s.Thresholds = engine.Thresholds{Medium: d.Thresholds.Medium, High: d.Thresholds.High}
	s.Clock = engine.Clock{Mode: d.Clock.Mode, FixedAt: d.Clock.FixedAt}
	for k, f := range d.Features { // …then what is stored wins
		s.Features[k] = engine.FeatureSetting{Weight: f.Weight, Enabled: f.Enabled}
	}
	return s, nil
}

func decodeRow(row store.SettingsRow) (Record, error) {
	s, err := decode(row.Data)
	if err != nil {
		return Record{}, err
	}
	return Record{Version: row.Version, UpdatedAt: row.UpdatedAt.UTC(), Settings: s}, nil
}
