package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/swaggest/swgui"
	"github.com/swaggest/swgui/v5emb"

	"github.com/mrDisa/Prolongo/backend-go/openapi"
)

const (
	docsPath = "/api/v1/docs"
	specPath = "/api/v1/openapi.yml"
)

// mountDocs serves Swagger UI and the raw spec. core does not check the session
// here: nginx closes both paths for anonymous users, and inside a session
// "Try it out" sends the session cookie automatically (same origin).
func mountDocs(r chi.Router) {
	ui := v5emb.NewHandlerWithConfig(swgui.Config{
		Title:       "Prolongo API",
		SwaggerJSON: specPath,
		BasePath:    docsPath + "/",
		SettingsUI: map[string]string{
			"docExpansion":             `"list"`,
			"defaultModelsExpandDepth": "0", // hide the long schema list at the bottom
			"displayRequestDuration":   "true",
			"tryItOutEnabled":          "true",
		},
	})
	r.Get(docsPath, ui.ServeHTTP)
	r.Get(docsPath+"/*", ui.ServeHTTP)
	r.Get(specPath, serveSpec)
}

func serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(openapi.Spec)
}
