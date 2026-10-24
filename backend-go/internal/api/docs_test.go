package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/openapi"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSpecEndpoint(t *testing.T) {
	h := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	rec := get(t, h, "/api/v1/openapi.yml")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/yaml; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, openapi.Spec, rec.Body.Bytes())
}

func TestSwaggerUI(t *testing.T) {
	h := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)))

	for _, path := range []string{"/api/v1/docs", "/api/v1/docs/"} {
		rec := get(t, h, path)
		require.Equal(t, http.StatusOK, rec.Code, path)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/html", path)
		assert.Contains(t, rec.Body.String(), "/api/v1/openapi.yml", path)
	}

	// Every script the page loads must be served from the embedded assets.
	page := get(t, h, "/api/v1/docs/").Body.String()
	scripts := regexp.MustCompile(`src="([^"]+\.js)"`).FindAllStringSubmatch(page, -1)
	require.NotEmpty(t, scripts, "page has no scripts")
	for _, m := range scripts {
		rec := get(t, h, m[1])
		assert.Equal(t, http.StatusOK, rec.Code, m[1])
		assert.NotZero(t, rec.Body.Len(), m[1])
	}
}
