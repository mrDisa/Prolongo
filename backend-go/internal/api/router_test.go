package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter(t *testing.T, level slog.Level) (http.Handler, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level}))
	return NewRouter(testDeps(t, logger)), &buf
}

func TestHealthz(t *testing.T) {
	h, _ := newTestRouter(t, slog.LevelInfo)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestUnknownRouteIs404(t *testing.T) {
	h, _ := newTestRouter(t, slog.LevelInfo)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"not_found","message":"Маршрут не найден"}}`, rec.Body.String())
}

func TestWrongMethodIs405(t *testing.T) {
	h, _ := newTestRouter(t, slog.LevelInfo)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/auth/login", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"method_not_allowed","message":"Метод не поддерживается"}}`, rec.Body.String())
}

func TestRequestLog(t *testing.T) {
	h, buf := newTestRouter(t, slog.LevelInfo)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nope", nil))

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "http request", line["msg"])
	assert.Equal(t, "GET", line["method"])
	assert.Equal(t, "/nope", line["path"])
	assert.EqualValues(t, 404, line["status"])
	assert.NotEmpty(t, line["request_id"])
	assert.Contains(t, line, "duration_ms")
}

func TestHealthzNotLoggedAtInfo(t *testing.T) {
	h, buf := newTestRouter(t, slog.LevelInfo)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Empty(t, buf.String())
}
