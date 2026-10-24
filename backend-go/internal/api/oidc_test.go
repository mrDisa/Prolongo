package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/oidc"
)

const fakeAuthorize = "http://localhost:8081/realms/prolongo/protocol/openid-connect/auth"

// fakeOIDC stands in for Keycloak: the protocol itself is tested in internal/oidc.
type fakeOIDC struct {
	authErr     error
	exchangeErr error
	started     oidc.Login // what AuthURL was given
	exchanged   oidc.Login // what Exchange was given
}

func (f *fakeOIDC) AuthURL(_ context.Context, l oidc.Login) (string, error) {
	f.started = l
	if f.authErr != nil {
		return "", f.authErr
	}
	return fakeAuthorize + "?state=" + l.State, nil
}

func (f *fakeOIDC) Exchange(_ context.Context, code string, l oidc.Login) (oidc.Identity, error) {
	f.exchanged = l
	if f.exchangeErr != nil {
		return oidc.Identity{}, f.exchangeErr
	}
	if code != "good-code" {
		return oidc.Identity{}, errors.New("invalid_grant")
	}
	return oidc.Identity{Subject: "sub-marketer", Username: "marketer", DisplayName: "Маркетолог Prolongo"}, nil
}

func oidcRouter(t *testing.T, f *fakeOIDC) http.Handler {
	t.Helper()
	d := testDeps(t, nil)
	d.OIDC = f
	return NewRouter(d)
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// startOIDC runs /auth/oidc/login and returns the attempt cookie.
func startOIDC(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(h, http.MethodGet, "/api/v1/auth/oidc/login", "", nil)
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	c := cookieNamed(rec, oidcCookie)
	require.NotNil(t, c)
	return c
}

func callback(h http.Handler, query string, attempt *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback"+query, nil)
	if attempt != nil {
		req.AddCookie(attempt)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestOIDCDisabled(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	for _, path := range []string{"/api/v1/auth/oidc/login", "/api/v1/auth/oidc/callback?code=x&state=y"} {
		rec := do(h, http.MethodGet, path, "", nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
		assert.JSONEq(t, `{"error":{"code":"not_found","message":"Вход через SSO выключен"}}`, rec.Body.String())
	}
}

func TestOIDCLoginRedirects(t *testing.T) {
	f := &fakeOIDC{}
	h := oidcRouter(t, f)
	rec := do(h, http.MethodGet, "/api/v1/auth/oidc/login", "", nil)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, fakeAuthorize+"?state="+f.started.State, rec.Header().Get("Location"))
	c := cookieNamed(rec, oidcCookie)
	require.NotNil(t, c)
	assert.Equal(t, f.started.State+"."+f.started.Nonce+"."+f.started.Verifier, c.Value)
	assert.Equal(t, "/api/v1/auth/oidc/", c.Path, "sent only to login and callback")
	assert.Equal(t, 300, c.MaxAge, "5 minutes")
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Nil(t, cookieNamed(rec, testCookie), "no session yet")
}

func TestOIDCProviderUnavailable(t *testing.T) {
	h := oidcRouter(t, &fakeOIDC{authErr: fmt.Errorf("%w: connection refused", oidc.ErrUnavailable)})
	rec := do(h, http.MethodGet, "/api/v1/auth/oidc/login", "", nil)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"unavailable","message":"Сервис входа SSO недоступен, попробуйте позже"}}`, rec.Body.String())
	assert.Nil(t, cookieNamed(rec, oidcCookie))
}

// The whole round trip: login → provider → callback → session → /auth/me says "oidc".
func TestOIDCCallbackOpensSession(t *testing.T) {
	f := &fakeOIDC{}
	h := oidcRouter(t, f)
	attempt := startOIDC(t, h)

	rec := callback(h, "?code=good-code&state="+f.started.State, attempt)
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.Equal(t, f.started, f.exchanged, "nonce and PKCE verifier came back from the cookie")

	cleared := cookieNamed(rec, oidcCookie)
	require.NotNil(t, cleared)
	assert.Negative(t, cleared.MaxAge, "the attempt cookie is deleted")

	session := cookieNamed(rec, testCookie)
	require.NotNil(t, session)
	assert.True(t, session.HttpOnly)
	me := do(h, http.MethodGet, "/api/v1/auth/me", "", session)
	require.Equal(t, http.StatusOK, me.Code)
	assert.JSONEq(t, `{"user":{"id":2,"username":"marketer","displayName":"Маркетолог Prolongo","authMethod":"oidc"},"oidcEnabled":true}`, me.Body.String())
	assert.Equal(t, http.StatusOK, do(h, http.MethodGet, "/internal/auth/verify", "", session).Code, "nginx lets the SSO user into ingest too")
}

func TestOIDCCallbackRefuses(t *testing.T) {
	const notStarted = "Вход через SSO не начат или устарел — начните заново"
	for name, tc := range map[string]struct {
		query    func(state string) string
		attempt  func(c *http.Cookie) *http.Cookie
		exchange error
		message  string
	}{
		"no attempt cookie": {func(s string) string { return "?code=good-code&state=" + s }, func(*http.Cookie) *http.Cookie { return nil }, nil, notStarted},
		"broken cookie": {func(s string) string { return "?code=good-code&state=" + s },
			func(c *http.Cookie) *http.Cookie { c.Value = "only-one-part"; return c }, nil, notStarted},
		"other state (CSRF)": {func(string) string { return "?code=good-code&state=forged" }, nil, nil, notStarted},
		"no code":            {func(s string) string { return "?state=" + s }, nil, nil, notStarted},
		"cancelled at the provider": {func(s string) string { return "?error=access_denied&state=" + s },
			nil, nil, "Вход через SSO отменён"},
		"token not accepted": {func(s string) string { return "?code=good-code&state=" + s }, nil,
			errors.New("oidc: verify id_token: expired"), "Не удалось подтвердить вход через SSO"},
		"unknown code": {func(s string) string { return "?code=stolen&state=" + s }, nil, nil, "Не удалось подтвердить вход через SSO"},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeOIDC{exchangeErr: tc.exchange}
			h := oidcRouter(t, f)
			attempt := startOIDC(t, h)
			if tc.attempt != nil {
				attempt = tc.attempt(attempt)
			}

			rec := callback(h, tc.query(f.started.State), attempt)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.JSONEq(t, `{"error":{"code":"unauthorized","message":"`+tc.message+`"}}`, rec.Body.String())
			assert.Nil(t, cookieNamed(rec, testCookie), "no session")
			assert.NotContains(t, rec.Body.String(), "expired", "details go to the log only")
		})
	}
}

// The attempt cookie value never contains a dot inside a part: otherwise the split would fail.
func TestOIDCCookieFormat(t *testing.T) {
	for range 50 {
		l := oidc.NewLogin()
		for _, part := range []string{l.State, l.Nonce, l.Verifier} {
			assert.False(t, strings.Contains(part, "."), part)
		}
	}
}
