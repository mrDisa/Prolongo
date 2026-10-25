package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SettingsRow is the single row of core.model_settings (or one version of its history;
// UpdatedAt is then the time the version was saved). Data is the settings JSON;
// its meaning belongs to package settings, the store only keeps it.
type SettingsRow struct {
	Version   int64
	UpdatedAt time.Time
	Data      []byte
}

// SettingsRow returns the stored settings, or ErrNotFound before the first start.
func (s *Store) SettingsRow(ctx context.Context) (SettingsRow, error) {
	var r SettingsRow
	err := s.pool.QueryRow(ctx, `SELECT version, updated_at, data FROM core.model_settings WHERE id = 1`).
		Scan(&r.Version, &r.UpdatedAt, &r.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return SettingsRow{}, ErrNotFound
	}
	if err != nil {
		return SettingsRow{}, fmt.Errorf("store: settings: %w", err)
	}
	return r, nil
}

// InsertSettings creates the row with version 1 unless it already exists, and returns the stored row.
// Two core instances starting at once cannot overwrite each other. Version 1 also starts the history.
func (s *Store) InsertSettings(ctx context.Context, data []byte) (SettingsRow, error) {
	_, err := s.pool.Exec(ctx, `
		WITH created AS (
			INSERT INTO core.model_settings (id, version, data) VALUES (1, 1, $1)
			ON CONFLICT (id) DO NOTHING
			RETURNING version, data, updated_at
		)
		INSERT INTO core.model_settings_history (version, data, saved_at)
		SELECT version, data, updated_at FROM created
		ON CONFLICT (version) DO NOTHING`, data)
	if err != nil {
		return SettingsRow{}, fmt.Errorf("store: insert settings: %w", err)
	}
	return s.SettingsRow(ctx)
}

// UpdateSettings changes the settings atomically: it locks the row (SELECT … FOR UPDATE),
// passes it to change, saves the returned data, increments the version and adds it to the history.
// Concurrent PATCH requests wait for each other instead of losing one of the changes.
// If change returns an error, nothing is saved and that error is returned as is.
func (s *Store) UpdateSettings(ctx context.Context, change func(SettingsRow) ([]byte, error)) (SettingsRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SettingsRow{}, fmt.Errorf("store: update settings: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var cur SettingsRow
	err = tx.QueryRow(ctx, `SELECT version, updated_at, data FROM core.model_settings WHERE id = 1 FOR UPDATE`).
		Scan(&cur.Version, &cur.UpdatedAt, &cur.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return SettingsRow{}, ErrNotFound
	}
	if err != nil {
		return SettingsRow{}, fmt.Errorf("store: update settings: %w", err)
	}

	data, err := change(cur)
	if err != nil {
		return SettingsRow{}, err
	}

	var next SettingsRow
	err = tx.QueryRow(ctx, `
		UPDATE core.model_settings SET data = $1, version = version + 1, updated_at = now()
		WHERE id = 1 RETURNING version, updated_at, data`, data).
		Scan(&next.Version, &next.UpdatedAt, &next.Data)
	if err != nil {
		return SettingsRow{}, fmt.Errorf("store: update settings: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO core.model_settings_history (version, data, saved_at) VALUES ($1, $2, $3)
		ON CONFLICT (version) DO NOTHING`, next.Version, next.Data, next.UpdatedAt)
	if err != nil {
		return SettingsRow{}, fmt.Errorf("store: settings history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SettingsRow{}, fmt.Errorf("store: update settings: %w", err)
	}
	return next, nil
}

// SettingsHistory returns the saved versions, newest first, at most limit of them.
func (s *Store) SettingsHistory(ctx context.Context, limit int) ([]SettingsRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT version, saved_at, data FROM core.model_settings_history
		ORDER BY version DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: settings history: %w", err)
	}
	defer rows.Close()
	versions := []SettingsRow{}
	for rows.Next() {
		var r SettingsRow
		if err := rows.Scan(&r.Version, &r.UpdatedAt, &r.Data); err != nil {
			return nil, fmt.Errorf("store: settings history: %w", err)
		}
		versions = append(versions, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: settings history: %w", err)
	}
	return versions, nil
}
