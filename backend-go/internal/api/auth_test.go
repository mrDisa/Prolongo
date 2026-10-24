package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every route behind requireSession. New protected routes (G8, G6…) go into this list.
var protectedRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/v1/auth/me"},
	{http.MethodPost, "/api/v1/auth/logout"},
	{http.MethodGet, "/api/v1/model/settings"},
	{http.MethodPatch, "/api/v1/model/settings"},
	{http.MethodPost, "/api/v1/model/settings/reset"},
	{http.MethodGet, "/api/v1/customers"},
	{http.MethodGet, "/api/v1/customers/5"},
	{http.MethodGet, "/api/v1/analytics/summary"},
	{http.MethodGet, "/api/v1/analytics/churn-timeline"},
	{http.MethodPost, "/api/v1/model/preview"},
	{http.MethodGet, "/api/v1/stream"},
	{http.MethodGet, "/metrics"}, // without a bearer token: the session decides
}

func TestLoginSetsSessionCookie(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"admin123"}`, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"user":{"id":1,"username":"admin","displayName":"Администратор","authMethod":"password"}}`, rec.Body.String())

	c := sessionCookie(rec)
	require.NotNil(t, c)
	assert.NotEmpty(t, c.Value)
	assert.Equal(t, "/", c.Path)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.False(t, c.Secure, "COOKIE_SECURE=false in tests")
	assert.InDelta(t, 12*3600, c.MaxAge, 5)
}

func TestLoginErrors(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cases := []struct {
		name, body string
		status     int
		want       string
	}{
		{"wrong password", `{"username":"admin","password":"nope"}`, 401,
			`{"error":{"code":"invalid_credentials","message":"Неверный логин или пароль"}}`},
		{"unknown user", `{"username":"ghost","password":"admin123"}`, 401,
			`{"error":{"code":"invalid_credentials","message":"Неверный логин или пароль"}}`},
		{"empty fields", `{"username":" ","password":""}`, 422,
			`{"error":{"code":"validation_error","message":"Заполните логин и пароль","details":{"username":["Обязательное поле"],"password":["Обязательное поле"]}}}`},
		{"not json", `username=admin`, 422,
			`{"error":{"code":"validation_error","message":"Ожидается JSON с полями username и password","details":{"body":["Некорректный JSON"]}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(h, http.MethodPost, "/api/v1/auth/login", c.body, nil)
			assert.Equal(t, c.status, rec.Code)
			assert.JSONEq(t, c.want, rec.Body.String())
			assert.Nil(t, sessionCookie(rec), "no cookie on failure")
		})
	}
}

func TestMe(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", login(t, h))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"user":{"id":1,"username":"admin","displayName":"Администратор","authMethod":"password"},"oidcEnabled":false}`, rec.Body.String())
}

func TestProtectedRoutesRequireSession(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	bad := []*http.Cookie{nil, {Name: testCookie, Value: ""}, {Name: testCookie, Value: "forged-token"}}

	for _, route := range protectedRoutes {
		for _, c := range bad {
			rec := do(h, route.method, route.path, "", c)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s cookie=%v", route.method, route.path, c)
			assert.JSONEq(t, `{"error":{"code":"unauthorized","message":"Требуется вход"}}`, rec.Body.String())
		}
	}
}

func TestLogoutEndsSession(t *testing.T) {
	h := NewRouter(testDeps(t, nil))
	cookie := login(t, h)

	rec := do(h, http.MethodPost, "/api/v1/auth/logout", "", cookie)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
	cleared := sessionCookie(rec)
	require.NotNil(t, cleared)
	assert.Empty(t, cleared.Value)
	assert.Negative(t, cleared.MaxAge, "Max-Age<0 tells the browser to delete the cookie")

	// The old cookie no longer works anywhere.
	for _, route := range protectedRoutes {
		assert.Equal(t, http.StatusUnauthorized, do(h, route.method, route.path, "", cookie).Code, route.path)
	}
	assert.Equal(t, http.StatusUnauthorized, do(h, http.MethodGet, "/internal/auth/verify", "", cookie).Code)
}

func TestVerify(t *testing.T) {
	h := NewRouter(testDeps(t, nil))

	rec := do(h, http.MethodGet, "/internal/auth/verify", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Body.String(), "nginx needs no body")

	rec = do(h, http.MethodGet, "/internal/auth/verify", "", login(t, h))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "admin", rec.Header().Get("X-Auth-User"))
	assert.Equal(t, "1", rec.Header().Get("X-Auth-User-Id"))
	assert.Empty(t, rec.Body.String())
}

func TestAuthConfigIsPublic(t *testing.T) {
	d := testDeps(t, nil)
	rec := do(NewRouter(d), http.MethodGet, "/api/v1/auth/config", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"oidcEnabled":false}`, rec.Body.String())

	d.OIDC = &fakeOIDC{}
	rec = do(NewRouter(d), http.MethodGet, "/api/v1/auth/config", "", nil)
	assert.JSONEq(t, `{"oidcEnabled":true}`, rec.Body.String())
}

func TestCookieSecureFlag(t *testing.T) {
	d := testDeps(t, nil)
	d.Cookie.Secure = true
	c := login(t, NewRouter(d))
	assert.True(t, c.Secure)
}
