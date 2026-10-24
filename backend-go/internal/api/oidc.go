package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/oidc"
)

// OIDCProvider is the single sign-on protocol (*oidc.Provider).
type OIDCProvider interface {
	AuthURL(ctx context.Context, l oidc.Login) (string, error)
	Exchange(ctx context.Context, code string, l oidc.Login) (oidc.Identity, error)
}

// The sign-in attempt waits in this cookie while the person is at the provider's login page.
const (
	oidcCookie     = "prolongo_oidc"
	oidcCookiePath = "/api/v1/auth/oidc/" // sent only to login and callback
	oidcCookieTTL  = 5 * time.Minute
)

// oidcLogin: GET /api/v1/auth/oidc/login — remember state, nonce and PKCE verifier,
// then send the browser to the provider (Keycloak).
func (h *handlers) oidcLogin(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		writeError(w, http.StatusNotFound, codeNotFound, "Вход через SSO выключен", nil)
		return
	}
	l := oidc.NewLogin()
	authURL, err := h.oidc.AuthURL(r.Context(), l)
	if errors.Is(err, oidc.ErrUnavailable) {
		h.logger.WarnContext(r.Context(), "oidc provider unavailable", slog.Any("error", err))
		writeError(w, http.StatusServiceUnavailable, codeUnavailable, "Сервис входа SSO недоступен, попробуйте позже", nil)
		return
	}
	if err != nil {
		writeInternal(w, r, h.logger, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oidcCookie,
		Value:    l.State + "." + l.Nonce + "." + l.Verifier, // base64url values: no dots inside
		Path:     oidcCookiePath,
		MaxAge:   int(oidcCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode, // sent on the top-level redirect back from the provider
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

// oidcCallback: GET /api/v1/auth/oidc/callback?code=…&state=… — the provider sends the browser
// back here. Check the state, exchange the code, verify the ID token, open a session, go to "/".
func (h *handlers) oidcCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		writeError(w, http.StatusNotFound, codeNotFound, "Вход через SSO выключен", nil)
		return
	}
	l, ok := readOIDCCookie(r)
	h.clearOIDCCookie(w) // one attempt, one callback: a replayed callback finds nothing
	q := r.URL.Query()
	switch {
	case !ok:
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "Вход через SSO не начат или устарел — начните заново", nil)
		return
	case q.Get("error") != "": // the person pressed "cancel" or the provider refused
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "Вход через SSO отменён", nil)
		return
	case subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(l.State)) != 1 || q.Get("code") == "":
		// another tab's callback or a forged link (CSRF): not our attempt
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "Вход через SSO не начат или устарел — начните заново", nil)
		return
	}

	id, err := h.oidc.Exchange(r.Context(), q.Get("code"), l)
	if err != nil {
		h.logger.WarnContext(r.Context(), "oidc sign-in refused", slog.Any("error", err))
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "Не удалось подтвердить вход через SSO", nil)
		return
	}
	user, sess, err := h.auth.LoginOIDC(r.Context(), id.Subject, id.Username, id.DisplayName)
	if err != nil {
		writeInternal(w, r, h.logger, err)
		return
	}
	h.logger.InfoContext(r.Context(), "oidc sign-in", slog.String("username", user.Username))
	h.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	http.Redirect(w, r, "/", http.StatusFound)
}

func readOIDCCookie(r *http.Request) (oidc.Login, bool) {
	c, err := r.Cookie(oidcCookie)
	if err != nil {
		return oidc.Login{}, false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return oidc.Login{}, false
	}
	return oidc.Login{State: parts[0], Nonce: parts[1], Verifier: parts[2]}, true
}

func (h *handlers) clearOIDCCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: oidcCookie, Value: "", Path: oidcCookiePath, MaxAge: -1,
		HttpOnly: true, Secure: h.cookie.Secure, SameSite: http.SameSiteLaxMode,
	})
}
