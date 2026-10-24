package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error codes from the API contract (openapi.yml, schema Error).
const (
	codeBadRequest         = "bad_request"
	codeUnauthorized       = "unauthorized"
	codeInvalidCredentials = "invalid_credentials"
	codeNotFound           = "not_found"
	codeMethodNotAllowed   = "method_not_allowed"
	codeValidation         = "validation_error"
	codeInternal           = "internal"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Details map[string][]string `json:"details,omitempty"`
}

// writeError writes {"error": {"code", "message", "details"}} with the given status.
func writeError(w http.ResponseWriter, status int, code, message string, details map[string][]string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message, Details: details}})
}

// writeInternal logs the real cause and returns a generic 500: internals never leak to the client.
func writeInternal(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	logger.ErrorContext(r.Context(), "request failed", slog.String("path", r.URL.Path), slog.Any("error", err))
	writeError(w, http.StatusInternalServerError, codeInternal, "Внутренняя ошибка сервера", nil)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, codeNotFound, "Маршрут не найден", nil)
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "Метод не поддерживается", nil)
}
