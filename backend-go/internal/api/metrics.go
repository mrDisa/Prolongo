package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// requireMetricsAccess guards /metrics: Prometheus sends "Authorization: Bearer <METRICS_TOKEN>",
// a person in the panel has the session cookie (openapi.yml: metricsBearer or cookieAuth).
func (h *handlers) requireMetricsAccess(next http.Handler) http.Handler {
	session := h.requireSession(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			session.ServeHTTP(w, r)
			return
		}
		if !h.validMetricsToken(token) {
			writeError(w, http.StatusUnauthorized, codeUnauthorized, "Неверный токен метрик", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validMetricsToken compares in constant time: the response time must not tell an attacker
// how many leading characters were right. Hashing first makes the lengths equal too.
// An empty METRICS_TOKEN turns bearer access off; the session still works.
func (h *handlers) validMetricsToken(token string) bool {
	if h.metricsToken == "" {
		return false
	}
	got, want := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(h.metricsToken))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// bearerToken reads "Authorization: Bearer <token>"; the scheme is case-insensitive (RFC 9110).
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return strings.TrimSpace(token), true
}
