// Package oidc is single sign-on through an OpenID Connect provider (Keycloak in compose),
// bonus task G12. It speaks the protocol only: cookies and redirects live in internal/api,
// users and sessions in internal/auth.
//
// The flow is "authorization code + PKCE": the browser is sent to the provider with a random
// state, nonce and code challenge; the provider sends it back with a one-time code; core
// exchanges the code (proving it knows the PKCE verifier) and checks the signed ID token.
package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Config comes from OIDC_* in .env.
type Config struct {
	IssuerURL    string // iss of the tokens, what the browser sees: http://localhost:8081/realms/prolongo
	InternalURL  string // where core reaches the provider: http://keycloak:8081/realms/prolongo; empty → IssuerURL
	ClientID     string
	ClientSecret string
	RedirectURL  string // our callback, registered at the provider
}

// Identity is the signed-in person as the provider vouches for them.
type Identity struct {
	Subject     string // stable id at the provider (sub): the key of the user
	Username    string // preferred_username, else email, else sub
	DisplayName string // name, else Username
}

// ErrUnavailable: the provider cannot be reached right now (discovery failed).
var ErrUnavailable = errors.New("oidc: provider unavailable")

const httpTimeout = 10 * time.Second

// Provider talks to the OIDC provider. Discovery runs on first use and is retried until it
// succeeds, so core starts even while Keycloak is still booting.
type Provider struct {
	cfg    Config
	client *http.Client

	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *gooidc.IDTokenVerifier
}

// New prepares a provider; it does not contact it yet.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg, client: &http.Client{Timeout: httpTimeout}}
}

// Login is what one sign-in attempt needs to remember until the callback (kept in a cookie).
type Login struct {
	State    string // against CSRF: the callback must bring the same value back
	Nonce    string // against replay: must come back inside the ID token
	Verifier string // PKCE: only we know it, so a stolen code is useless to others
}

// NewLogin makes fresh random values for one attempt.
func NewLogin() Login {
	return Login{State: random(), Nonce: random(), Verifier: oauth2.GenerateVerifier()}
}

// AuthURL is the provider's login page for this attempt.
func (p *Provider) AuthURL(ctx context.Context, l Login) (string, error) {
	oc, _, err := p.setup(ctx)
	if err != nil {
		return "", err
	}
	return oc.AuthCodeURL(l.State, gooidc.Nonce(l.Nonce), oauth2.S256ChallengeOption(l.Verifier)), nil
}

// Exchange turns the code from the callback into a verified identity: it checks the ID token's
// signature, issuer, audience (our client id), expiry and nonce.
func (p *Provider) Exchange(ctx context.Context, code string, l Login) (Identity, error) {
	oc, verifier, err := p.setup(ctx)
	if err != nil {
		return Identity{}, err
	}
	ctx = gooidc.ClientContext(ctx, p.client)
	token, err := oc.Exchange(ctx, code, oauth2.VerifierOption(l.Verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("oidc: exchange code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return Identity{}, errors.New("oidc: no id_token in the token response")
	}
	idt, err := verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("oidc: verify id_token: %w", err)
	}
	if idt.Nonce != l.Nonce {
		return Identity{}, errors.New("oidc: id_token nonce does not match")
	}

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Name              string `json:"name"`
	}
	if err := idt.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("oidc: read claims: %w", err)
	}
	id := Identity{Subject: idt.Subject, Username: firstNonEmpty(claims.PreferredUsername, claims.Email, idt.Subject)}
	id.DisplayName = firstNonEmpty(claims.Name, id.Username)
	return id, nil
}

// setup runs discovery once. Tokens carry the public issuer, but core fetches the discovery
// document from the internal address; InsecureIssuerURLContext tells go-oidc to expect the
// public issuer inside it. The token and keys endpoints then point at the internal address
// (Keycloak --hostname-backchannel-dynamic), the login page at the public one.
func (p *Provider) setup(ctx context.Context) (*oauth2.Config, *gooidc.IDTokenVerifier, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.oauth != nil {
		return p.oauth, p.verifier, nil
	}

	discovery := p.cfg.InternalURL
	if discovery == "" {
		discovery = p.cfg.IssuerURL
	}
	dctx := gooidc.ClientContext(ctx, p.client) // the key set fetched later reuses this client
	if discovery != p.cfg.IssuerURL {
		dctx = gooidc.InsecureIssuerURLContext(dctx, p.cfg.IssuerURL)
	}
	dctx, cancel := context.WithTimeout(dctx, httpTimeout)
	defer cancel()
	provider, err := gooidc.NewProvider(dctx, discovery)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	p.oauth = &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}
	p.verifier = provider.Verifier(&gooidc.Config{ClientID: p.cfg.ClientID}) // checks aud = our client id
	return p.oauth, p.verifier, nil
}

func random() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b) // never fails (crypto/rand, Go 1.24+)
	return base64.RawURLEncoding.EncodeToString(b)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
