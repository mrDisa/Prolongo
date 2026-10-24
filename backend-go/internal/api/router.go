// Package api wires the HTTP routes of core.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
)

// Deps is everything the HTTP layer needs. main builds it once at startup.
type Deps struct {
	Logger      *slog.Logger
	Auth        *auth.Service
	Cookie      CookieConfig
	OIDCEnabled bool
}

type handlers struct {
	logger      *slog.Logger
	auth        *auth.Service
	cookie      CookieConfig
	oidcEnabled bool
}

// NewRouter builds the HTTP handler with common middleware and all routes.
func NewRouter(d Deps) http.Handler {
	h := &handlers{logger: d.Logger, auth: d.Auth, cookie: d.Cookie, oidcEnabled: d.OIDCEnabled}

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

		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Get("/auth/me", h.me)
			r.Post("/auth/logout", h.logout)
		})
	})

	return r
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
