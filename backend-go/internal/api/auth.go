package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
)

// CookieConfig describes the session cookie (SESSION_COOKIE_NAME, COOKIE_SECURE).
type CookieConfig struct {
	Name   string
	Secure bool
}

const maxLoginBody = 1 << 10 // 1 KiB is plenty for username + password

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userJSON struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AuthMethod  string `json:"authMethod"`
}

func toUserJSON(u auth.User) userJSON {
	return userJSON{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, AuthMethod: u.AuthMethod}
}

// login: POST /api/v1/auth/login.
func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Ожидается JSON с полями username и password",
			map[string][]string{"body": {"Некорректный JSON"}})
		return
	}
	details := map[string][]string{}
	if strings.TrimSpace(req.Username) == "" {
		details["username"] = []string{"Обязательное поле"}
	}
	if req.Password == "" {
		details["password"] = []string{"Обязательное поле"}
	}
	if len(details) > 0 {
		writeError(w, http.StatusUnprocessableEntity, codeValidation, "Заполните логин и пароль", details)
		return
	}

	user, sess, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials, "Неверный логин или пароль", nil)
		return
	}
	if err != nil {
		writeInternal(w, r, h.logger, err)
		return
	}
	h.setSessionCookie(w, sess.Token, sess.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserJSON(user)})
}

// me: GET /api/v1/auth/me (behind requireSession).
func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserJSON(user), "oidcEnabled": h.oidcEnabled})
}

// logout: POST /api/v1/auth/logout (behind requireSession).
func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(h.cookie.Name); err == nil {
		if err := h.auth.Logout(r.Context(), c.Value); err != nil {
			writeInternal(w, r, h.logger, err)
			return
		}
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// verify: GET /internal/auth/verify, called by nginx auth_request before every ingest request.
// 200 + X-Auth-User headers or 401, both without a body.
func (h *handlers) verify(w http.ResponseWriter, r *http.Request) {
	user, err := h.sessionUser(r)
	if err != nil {
		if !errors.Is(err, auth.ErrNoSession) {
			h.logger.ErrorContext(r.Context(), "session check failed", "error", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("X-Auth-User", user.Username)
	w.Header().Set("X-Auth-User-Id", strconv.FormatInt(user.ID, 10))
	w.WriteHeader(http.StatusOK)
}

// requireSession lets the request through only with a live session and puts the user into the context.
func (h *handlers) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.sessionUser(r)
		if errors.Is(err, auth.ErrNoSession) {
			writeError(w, http.StatusUnauthorized, codeUnauthorized, "Требуется вход", nil)
			return
		}
		if err != nil {
			writeInternal(w, r, h.logger, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), user)))
	})
}

func (h *handlers) sessionUser(r *http.Request) (auth.User, error) {
	c, err := r.Cookie(h.cookie.Name)
	if err != nil {
		return auth.User{}, auth.ErrNoSession
	}
	return h.auth.Authenticate(r.Context(), c.Value)
}

func (h *handlers) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true, // JavaScript cannot read it: an XSS bug cannot steal the session
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode, // not sent on cross-site POSTs: basic CSRF protection
	})
}

func (h *handlers) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
