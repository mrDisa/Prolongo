// Package api wires the HTTP routes of core.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
)

// Deps is everything the HTTP layer needs. main builds it once at startup.
type Deps struct {
	Logger      *slog.Logger
	Auth        *auth.Service
	Settings    *settings.Service
	Cookie      CookieConfig
	OIDCEnabled bool
}

type handlers struct {
	logger      *slog.Logger
	auth        *auth.Service
	settings    *settings.Service
	cookie      CookieConfig
	oidcEnabled bool
}

// NewRouter builds the HTTP handler with common middleware and all routes.
func NewRouter(d Deps) http.Handler {
	h := &handlers{logger: d.Logger, auth: d.Auth, settings: d.Settings, cookie: d.Cookie, oidcEnabled: d.OIDCEnabled}

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
		})
	})

	return r
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
