package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The issuer the tokens carry. Nothing listens there: like in compose, core must reach the
// provider through the internal address only.
const publicIssuer = "http://localhost:8081/realms/prolongo"

// fakeProvider is a minimal OpenID provider: discovery, keys, and a token endpoint that
// checks the client secret and PKCE and signs an ID token.
type fakeProvider struct {
	t    *testing.T
	srv  *httptest.Server
	key  *rsa.PrivateKey
	down atomic.Bool

	mu    sync.Mutex
	codes map[string]pendingCode // code → what the authorize step saw

	// what the next ID token looks like; tests change it
	claims func(base map[string]any)
	signer *rsa.PrivateKey // nil → the published key
}

type pendingCode struct{ challenge, nonce string }

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &fakeProvider{t: t, key: key, codes: map[string]pendingCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /certs", f.certs)
	mux.HandleFunc("POST /token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeProvider) discovery(w http.ResponseWriter, _ *http.Request) {
	if f.down.Load() {
		http.Error(w, "starting", http.StatusServiceUnavailable)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                publicIssuer,
		"authorization_endpoint":                publicIssuer + "/protocol/openid-connect/auth", // the browser's address
		"token_endpoint":                        f.srv.URL + "/token",                           // core's address
		"jwks_uri":                              f.srv.URL + "/certs",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *fakeProvider) certs(w http.ResponseWriter, _ *http.Request) {
	_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
	}})
}

// authorize plays the provider's login page: it remembers the challenge and nonce of the
// URL core built and returns the code the browser would bring back.
func (f *fakeProvider) authorize(authURL string) (code, state string) {
	u, err := url.Parse(authURL)
	require.NoError(f.t, err)
	q := u.Query()
	code = "code-" + random()
	f.mu.Lock()
	f.codes[code] = pendingCode{challenge: q.Get("code_challenge"), nonce: q.Get("nonce")}
	f.mu.Unlock()
	return code, q.Get("state")
}

func (f *fakeProvider) token(w http.ResponseWriter, r *http.Request) {
	require.NoError(f.t, r.ParseForm())
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != "prolongo" || secret != "prolongo-secret" {
		tokenError(w, "invalid_client")
		return
	}
	f.mu.Lock()
	pc, known := f.codes[r.PostForm.Get("code")]
	delete(f.codes, r.PostForm.Get("code")) // a code works once
	f.mu.Unlock()
	if r.PostForm.Get("grant_type") != "authorization_code" || !known {
		tokenError(w, "invalid_grant")
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != pc.challenge {
		tokenError(w, "invalid_grant") // PKCE: the verifier does not match the challenge
		return
	}

	now := time.Now()
	claims := map[string]any{
		"iss": publicIssuer, "aud": "prolongo", "sub": "f3b1c2d4-0000-4000-8000-000000000001",
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "nonce": pc.nonce,
		"preferred_username": "marketer", "name": "Маркетолог Prolongo", "email": "marketer@example.test",
	}
	if f.claims != nil {
		f.claims(claims)
	}
	w.Header().Set("Content-Type", "application/json") // without it oauth2 parses the body as a form
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "access", "token_type": "Bearer", "expires_in": 300, "id_token": f.sign(claims),
	})
}

func (f *fakeProvider) sign(claims map[string]any) string {
	key := f.key
	if f.signer != nil {
		key = f.signer
	}
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	require.NoError(f.t, err)
	payload, _ := json.Marshal(claims)
	obj, err := s.Sign(payload)
	require.NoError(f.t, err)
	out, err := obj.CompactSerialize()
	require.NoError(f.t, err)
	return out
}

func tokenError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func (f *fakeProvider) provider() *Provider {
	return New(Config{
		IssuerURL: publicIssuer, InternalURL: f.srv.URL,
		ClientID: "prolongo", ClientSecret: "prolongo-secret",
		RedirectURL: "http://localhost:8080/api/v1/auth/oidc/callback",
	})
}

func TestLoginFlow(t *testing.T) {
	ctx := context.Background()
	f := newFakeProvider(t)
	p := f.provider()
	l := NewLogin()

	authURL, err := p.AuthURL(ctx, l)
	require.NoError(t, err)
	u, _ := url.Parse(authURL)
	assert.Equal(t, publicIssuer+"/protocol/openid-connect/auth", u.Scheme+"://"+u.Host+u.Path, "the browser goes to the public address")
	q := u.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "prolongo", q.Get("client_id"))
	assert.Equal(t, "http://localhost:8080/api/v1/auth/oidc/callback", q.Get("redirect_uri"))
	assert.Equal(t, "openid profile email", q.Get("scope"))
	assert.Equal(t, l.State, q.Get("state"))
	assert.Equal(t, l.Nonce, q.Get("nonce"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotContains(t, authURL, l.Verifier, "the verifier never leaves core")

	code, state := f.authorize(authURL)
	assert.Equal(t, l.State, state)
	id, err := p.Exchange(ctx, code, l)
	require.NoError(t, err)
	assert.Equal(t, Identity{Subject: "f3b1c2d4-0000-4000-8000-000000000001", Username: "marketer", DisplayName: "Маркетолог Prolongo"}, id)

	_, err = p.Exchange(ctx, code, l)
	assert.Error(t, err, "a code works once")
}

// Every check of the ID token and of PKCE must hold; one failing is enough to refuse.
func TestExchangeRefuses(t *testing.T) {
	ctx := context.Background()
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		setup  func(f *fakeProvider, l *Login)
		errHas string
	}{
		"wrong PKCE verifier": {func(_ *fakeProvider, l *Login) { l.Verifier = NewLogin().Verifier }, "invalid_grant"},
		"wrong nonce":         {func(_ *fakeProvider, l *Login) { l.Nonce = "other" }, "nonce"},
		"other audience":      {func(f *fakeProvider, _ *Login) { f.claims = func(c map[string]any) { c["aud"] = "grafana" } }, "audience"},
		"other issuer": {func(f *fakeProvider, _ *Login) {
			f.claims = func(c map[string]any) { c["iss"] = "http://evil/realms/x" }
		}, "different provider"},
		"expired": {func(f *fakeProvider, _ *Login) {
			f.claims = func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }
		}, "expired"},
		"signed by another key": {func(f *fakeProvider, _ *Login) { f.signer = otherKey }, "signature"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeProvider(t)
			p := f.provider()
			l := NewLogin()
			authURL, err := p.AuthURL(ctx, l)
			require.NoError(t, err)
			code, _ := f.authorize(authURL)
			tc.setup(f, &l)

			_, err = p.Exchange(ctx, code, l)
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), tc.errHas)
		})
	}
}

func TestClaimsFallback(t *testing.T) {
	ctx := context.Background()
	f := newFakeProvider(t)
	f.claims = func(c map[string]any) { delete(c, "preferred_username"); delete(c, "name") }
	p := f.provider()
	l := NewLogin()
	authURL, _ := p.AuthURL(ctx, l)
	code, _ := f.authorize(authURL)

	id, err := p.Exchange(ctx, code, l)
	require.NoError(t, err)
	assert.Equal(t, "marketer@example.test", id.Username)
	assert.Equal(t, "marketer@example.test", id.DisplayName)
}

// Keycloak boots slower than core: discovery fails until it is up, then works without a restart.
func TestDiscoveryRetries(t *testing.T) {
	ctx := context.Background()
	f := newFakeProvider(t)
	f.down.Store(true)
	p := f.provider()

	_, err := p.AuthURL(ctx, NewLogin())
	assert.True(t, errors.Is(err, ErrUnavailable), err)

	f.down.Store(false)
	_, err = p.AuthURL(ctx, NewLogin())
	assert.NoError(t, err)
}

// Without the internal address the issuer must match the discovery document exactly.
func TestIssuerMismatch(t *testing.T) {
	f := newFakeProvider(t)
	p := New(Config{IssuerURL: f.srv.URL, ClientID: "prolongo", ClientSecret: "prolongo-secret"})
	_, err := p.AuthURL(context.Background(), NewLogin())
	require.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "issuer")
}

func TestNewLoginIsRandom(t *testing.T) {
	a, b := NewLogin(), NewLogin()
	assert.NotEqual(t, a.State, b.State)
	assert.NotEqual(t, a.Nonce, b.Nonce)
	assert.NotEqual(t, a.Verifier, b.Verifier)
	assert.Len(t, a.State, 32, "24 random bytes in base64url")
}
