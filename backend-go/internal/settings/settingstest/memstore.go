// Package settingstest provides an in-memory settings.Store for tests without PostgreSQL.
package settingstest

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

// MemStore keeps the single settings row and its history in memory. Safe for concurrent use.
type MemStore struct {
	mu      sync.Mutex
	row     *store.SettingsRow
	history []store.SettingsRow // oldest first
}

func (m *MemStore) SettingsRow(_ context.Context) (store.SettingsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		return store.SettingsRow{}, store.ErrNotFound
	}
	return m.copyRow(), nil
}

func (m *MemStore) InsertSettings(_ context.Context, data []byte) (store.SettingsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		m.row = &store.SettingsRow{Version: 1, UpdatedAt: time.Now(), Data: slices.Clone(data)}
		m.history = append(m.history, m.copyRow())
	}
	return m.copyRow(), nil
}

func (m *MemStore) UpdateSettings(_ context.Context, change func(store.SettingsRow) ([]byte, error)) (store.SettingsRow, error) {
	m.mu.Lock() // the lock plays the role of SELECT … FOR UPDATE
	defer m.mu.Unlock()
	if m.row == nil {
		return store.SettingsRow{}, store.ErrNotFound
	}
	data, err := change(m.copyRow())
	if err != nil {
		return store.SettingsRow{}, err
	}
	m.row = &store.SettingsRow{Version: m.row.Version + 1, UpdatedAt: time.Now(), Data: slices.Clone(data)}
	m.history = append(m.history, m.copyRow())
	return m.copyRow(), nil
}

func (m *MemStore) SettingsHistory(_ context.Context, limit int) ([]store.SettingsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []store.SettingsRow{}
	for i := len(m.history) - 1; i >= 0 && len(out) < limit; i-- {
		r := m.history[i]
		r.Data = slices.Clone(r.Data)
		out = append(out, r)
	}
	return out, nil
}

// Data returns the stored JSON (nil before Ensure).
func (m *MemStore) Data() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		return nil
	}
	return slices.Clone(m.row.Data)
}

// SetData replaces the stored JSON, e.g. to simulate settings saved by an older version.
func (m *MemStore) SetData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.row = &store.SettingsRow{Version: 1, UpdatedAt: time.Now(), Data: slices.Clone(data)}
	m.history = []store.SettingsRow{m.copyRow()}
}

func (m *MemStore) copyRow() store.SettingsRow {
	r := *m.row
	r.Data = slices.Clone(r.Data)
	return r
}
