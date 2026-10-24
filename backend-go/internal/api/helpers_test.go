package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
	"github.com/mrDisa/Prolongo/backend-go/internal/auth/authtest"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings/settingstest"
)

const testCookie = "prolongo_session"

// testDeps builds router dependencies on top of an in-memory store with admin / admin123.
func testDeps(t *testing.T, logger *slog.Logger) Deps {
	t.Helper()
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	svc, err := auth.NewService(authtest.NewMemStore(), 12*time.Hour)
	require.NoError(t, err)
	require.NoError(t, svc.EnsureAdmin(context.Background(), "admin", "admin123"))
	settingsSvc := settings.NewService(&settingstest.MemStore{})
	_, err = settingsSvc.Ensure(context.Background())
	require.NoError(t, err)
	return Deps{Logger: logger, Auth: svc, Settings: settingsSvc, Cookie: CookieConfig{Name: testCookie}}
}

// do sends a request; body may be "", cookie may be nil.
func do(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie returns the session cookie set by a response, or nil.
func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == testCookie {
			return c
		}
	}
	return nil
}

// login signs in as admin and returns the session cookie.
func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"admin123"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	c := sessionCookie(rec)
	require.NotNil(t, c)
	return c
}
