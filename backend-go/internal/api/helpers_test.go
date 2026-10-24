package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
	"github.com/mrDisa/Prolongo/backend-go/internal/auth/authtest"
	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
	"github.com/mrDisa/Prolongo/backend-go/internal/recalc"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings/settingstest"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
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
	return Deps{
		Logger: logger, Auth: svc, Settings: settingsSvc, Cookie: CookieConfig{Name: testCookie},
		Current: referenceSnapshot(t), History: fakeHistory{},
	}
}

// staticSnapshot serves one fixed calculation.
type staticSnapshot struct{ snap *recalc.Snapshot }

func (s staticSnapshot) Current() *recalc.Snapshot { return s.snap }

// referenceSnapshot is the calculation on the organizers' data.json with default settings,
// as the worker would publish it (calcVersion 17 as in the openapi.yml examples).
func referenceSnapshot(t *testing.T) staticSnapshot {
	t.Helper()
	f, err := os.Open("../../../data/test-data/data.json")
	require.NoError(t, err)
	defer f.Close()
	ds, err := domain.FromJSON(f)
	require.NoError(t, err)
	s := engine.DefaultSettings()
	res := engine.Calculate(ds, s, engine.Now(s.Clock, ds, time.Now()))
	return staticSnapshot{&recalc.Snapshot{
		Result: res, Version: 17, CalculatedAt: time.Date(2026, 10, 19, 14, 35, 54, 204_000_000, time.UTC),
		Reason: recalc.ReasonStartup, Dataset: ds, SettingsVersion: 1,
	}}
}

// fakeHistory has one status change for customer 5.
type fakeHistory struct{}

func (fakeHistory) StatusHistory(_ context.Context, accountID int64, _ int) ([]store.StatusEvent, error) {
	if accountID != 5 {
		return []store.StatusEvent{}, nil
	}
	from := "active"
	return []store.StatusEvent{{ID: 1, ServiceID: 25, AccountID: 5, FromStatus: &from, ToStatus: "prolong_wait",
		OccurredAt: time.Date(2026, 10, 19, 15, 10, 0, 0, time.UTC), Source: "simulator"}}, nil
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
