// Package api wires the HTTP routes of core.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
	"github.com/mrDisa/Prolongo/backend-go/internal/live"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
)

// Deps is everything the HTTP layer needs. main builds it once at startup.
type Deps struct {
	Logger      *slog.Logger
	Auth        *auth.Service
	Settings    *settings.Service
	Recalc      Refresher // nil in tests that do not need recalculation
	Current     SnapshotSource
	History     HistorySource
	Hub         *live.Hub
	Ping        time.Duration // SSE keep-alive; 0 → 15 s
	Cookie      CookieConfig
	OIDCEnabled bool
}

// Refresher recalculates the model and waits for the result (*recalc.Worker).
type Refresher interface {
	Refresh(ctx context.Context, reason string, timeout time.Duration) (*recalc.Snapshot, error)
}

type handlers struct {
	logger      *slog.Logger
	auth        *auth.Service
	settings    *settings.Service
	recalc      Refresher
	current     SnapshotSource
	history     HistorySource
	horizons    *horizonCache
	hub         *live.Hub
	ping        time.Duration
	cookie      CookieConfig
	oidcEnabled bool
}

// NewRouter builds the HTTP handler with common middleware and all routes.
func NewRouter(d Deps) http.Handler {
	h := &handlers{
		logger: d.Logger, auth: d.Auth, settings: d.Settings, recalc: d.Recalc,
		current: d.Current, history: d.History, horizons: &horizonCache{},
		hub: d.Hub, ping: d.Ping, cookie: d.Cookie, oidcEnabled: d.OIDCEnabled,
	}
	if h.ping <= 0 {
		h.ping = defaultPing
	}
	if h.hub == nil {
		h.hub = live.NewHub()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(d.Logger))
	r.Use(middleware.Recoverer)
	r.NotFound(notFound)
	r.MethodNotAllowed(methodNotAllowed)

	r.Get("/healthz", health)                // no auth: used by the compose healthcheck
	r.Get("/internal/auth/verify", h.verify) // nginx only, not proxied to the outside
	mountDocs(r)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", h.login)
		r.Get("/auth/config", h.authConfig) // public: the login screen has no session

		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Get("/auth/me", h.me)
			r.Post("/auth/logout", h.logout)
			r.Get("/model/settings", h.getSettings)
			r.Patch("/model/settings", h.patchSettings)
			r.Post("/model/settings/reset", h.resetSettings)
			r.Post("/model/preview", h.preview)
			r.Get("/customers", h.listCustomers)
			r.Get("/customers/{id}", h.getCustomer)
			r.Get("/analytics/summary", h.summary)
			r.Get("/stream", h.stream)
		})
	})

	return r
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
