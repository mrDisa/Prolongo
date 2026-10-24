// Package config loads core settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds every setting core reads from the environment.
type Config struct {
	HTTPAddr    string `env:"HTTP_ADDR" envDefault:":8000"`
	DatabaseURL string `env:"DATABASE_URL,required,notEmpty"`

	AdminUsername     string `env:"ADMIN_USERNAME" envDefault:"admin"`
	AdminPassword     string `env:"ADMIN_PASSWORD" envDefault:"admin123"`
	SessionTTLHours   int    `env:"SESSION_TTL_HOURS" envDefault:"12"`
	SessionCookieName string `env:"SESSION_COOKIE_NAME" envDefault:"prolongo_session"`
	CookieSecure      bool   `env:"COOKIE_SECURE" envDefault:"false"`

	MetricsToken string `env:"METRICS_TOKEN"`

	RecalcIntervalSeconds int `env:"RECALC_INTERVAL_SECONDS" envDefault:"60"`
	RecalcDebounceMS      int `env:"RECALC_DEBOUNCE_MS" envDefault:"300"`

	OIDC OIDC

	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

// OIDC is the optional single sign-on setup (bonus task G12).
type OIDC struct {
	Enabled      bool   `env:"OIDC_ENABLED" envDefault:"false"`
	IssuerURL    string `env:"OIDC_ISSUER_URL"`   // what the browser sees and the tokens carry as iss
	InternalURL  string `env:"OIDC_INTERNAL_URL"` // how core reaches the provider; empty → IssuerURL
	ClientID     string `env:"OIDC_CLIENT_ID"`
	ClientSecret string `env:"OIDC_CLIENT_SECRET"`
	RedirectURL  string `env:"OIDC_REDIRECT_URL"`
}

// Load reads the environment and validates the result.
func Load() (Config, error) {
	return parse(env.Options{})
}

func parse(opts env.Options) (Config, error) {
	var cfg Config
	if err := env.ParseWithOptions(&cfg, opts); err != nil {
		return Config{}, fmt.Errorf("config: %w", withEnvNames(err))
	}
	if _, err := cfg.SlogLevel(); err != nil {
		return Config{}, err
	}
	if cfg.RecalcIntervalSeconds < 1 {
		return Config{}, fmt.Errorf("config: RECALC_INTERVAL_SECONDS must be at least 1, got %d", cfg.RecalcIntervalSeconds)
	}
	if cfg.RecalcDebounceMS < 0 {
		return Config{}, fmt.Errorf("config: RECALC_DEBOUNCE_MS must not be negative, got %d", cfg.RecalcDebounceMS)
	}
	if cfg.OIDC.Enabled {
		var missing []string
		for _, v := range []struct{ name, value string }{
			{"OIDC_ISSUER_URL", cfg.OIDC.IssuerURL}, {"OIDC_CLIENT_ID", cfg.OIDC.ClientID},
			{"OIDC_CLIENT_SECRET", cfg.OIDC.ClientSecret}, {"OIDC_REDIRECT_URL", cfg.OIDC.RedirectURL},
		} {
			if v.value == "" {
				missing = append(missing, v.name)
			}
		}
		if len(missing) > 0 {
			return Config{}, fmt.Errorf("config: OIDC_ENABLED=1 needs %s", strings.Join(missing, ", "))
		}
	}
	return cfg, nil
}

// withEnvNames rewrites parse errors so they name the variable to fix
// (SESSION_TTL_HOURS) instead of the Go field (SessionTTLHours).
func withEnvNames(err error) error {
	var agg env.AggregateError
	if !errors.As(err, &agg) {
		return err
	}
	keys := envKeys(reflect.TypeOf(Config{}))
	msgs := make([]string, 0, len(agg.Errors))
	for _, e := range agg.Errors {
		var pe env.ParseError
		if errors.As(e, &pe) {
			if key, ok := keys[pe.Name]; ok {
				msgs = append(msgs, fmt.Sprintf("%s: invalid %s value: %v", key, pe.Type, pe.Err))
				continue
			}
		}
		msgs = append(msgs, e.Error())
	}
	return errors.New(strings.Join(msgs, "; "))
}

// envKeys maps struct field names to their env variable names, including nested structs.
func envKeys(t reflect.Type) map[string]string {
	keys := map[string]string{}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Struct {
			maps.Copy(keys, envKeys(f.Type))
			continue
		}
		if tag := f.Tag.Get("env"); tag != "" {
			keys[f.Name] = strings.Split(tag, ",")[0]
		}
	}
	return keys
}

// SlogLevel converts LOG_LEVEL (debug, info, warn, error) to a slog level.
func (c Config) SlogLevel() (slog.Level, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToLower(c.LogLevel))); err != nil {
		return 0, fmt.Errorf("config: LOG_LEVEL %q: use debug, info, warn or error", c.LogLevel)
	}
	return lvl, nil
}
