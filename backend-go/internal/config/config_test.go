package config

import (
	"log/slog"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func load(t *testing.T, vars map[string]string) (Config, error) {
	t.Helper()
	return parse(env.Options{Environment: vars})
}

func TestDefaults(t *testing.T) {
	cfg, err := load(t, map[string]string{"DATABASE_URL": "postgres://u:p@db:5432/x"})
	require.NoError(t, err)

	assert.Equal(t, ":8000", cfg.HTTPAddr)
	assert.Equal(t, "admin", cfg.AdminUsername)
	assert.Equal(t, "admin123", cfg.AdminPassword)
	assert.Equal(t, 12, cfg.SessionTTLHours)
	assert.Equal(t, "prolongo_session", cfg.SessionCookieName)
	assert.False(t, cfg.CookieSecure)
	assert.Equal(t, 60, cfg.RecalcIntervalSeconds)
	assert.Equal(t, 300, cfg.RecalcDebounceMS)
	assert.False(t, cfg.OIDC.Enabled)

	lvl, err := cfg.SlogLevel()
	require.NoError(t, err)
	assert.Equal(t, slog.LevelInfo, lvl)
}

func TestOverrides(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"DATABASE_URL":  "postgres://u:p@db:5432/x",
		"HTTP_ADDR":     ":9000",
		"COOKIE_SECURE": "true",
		"OIDC_ENABLED":  "1",
		"LOG_LEVEL":     "DEBUG",
	})
	require.NoError(t, err)

	assert.Equal(t, ":9000", cfg.HTTPAddr)
	assert.True(t, cfg.CookieSecure)
	assert.True(t, cfg.OIDC.Enabled)
	lvl, _ := cfg.SlogLevel()
	assert.Equal(t, slog.LevelDebug, lvl)
}

func TestDatabaseURLRequired(t *testing.T) {
	_, err := load(t, map[string]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")

	_, err = load(t, map[string]string{"DATABASE_URL": ""})
	require.Error(t, err)
}

func TestBadValues(t *testing.T) {
	_, err := load(t, map[string]string{"DATABASE_URL": "x", "LOG_LEVEL": "loud"})
	assert.ErrorContains(t, err, "LOG_LEVEL")

	_, err = load(t, map[string]string{"DATABASE_URL": "x", "SESSION_TTL_HOURS": "twelve"})
	assert.ErrorContains(t, err, "SESSION_TTL_HOURS")

	_, err = load(t, map[string]string{"DATABASE_URL": "x", "RECALC_INTERVAL_SECONDS": "0"})
	assert.ErrorContains(t, err, "RECALC_INTERVAL_SECONDS", "time.NewTicker panics on 0")

	_, err = load(t, map[string]string{"DATABASE_URL": "x", "RECALC_DEBOUNCE_MS": "-5"})
	assert.ErrorContains(t, err, "RECALC_DEBOUNCE_MS")
}
