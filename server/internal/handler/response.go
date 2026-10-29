package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	mw "github.com/auto-code-os/auto-code-os/server/internal/middleware"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/baby-hero-404/gocore/stdlog"
	"gorm.io/gorm"
)

type envelope map[string]any

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, envelope{"error": msg})
}

// writeServiceError maps a service error to an HTTP status and the {"error": msg} body. Client
// errors keep their message (it was written for the caller); 404 is a fixed "not found"; a 5xx never
// exposes the underlying error text (driver messages, paths, hostnames): the client gets a generic
// message and the cause is logged once with the request ID.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status := statusForError(err)
	msg := err.Error()
	switch {
	case status == http.StatusNotFound:
		msg = "not found"
	case status >= http.StatusInternalServerError:
		stdlog.WithContext(r.Context()).Error("request failed",
			"method", r.Method, "path", r.URL.Path, "status", status, "error", err.Error())
		msg = "internal server error"
	}
	writeError(w, status, msg)
}

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// clientIP resolves the originating client address for audit logging through the trusted-proxy
// aware resolver (mw.ClientIPResolver). Forwarding headers are never read here: only those added by
// configured trusted proxies count, so a client cannot choose what the audit log records.
func clientIP(r *http.Request) string { return mw.ClientIP(r) }

func statusForError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, service.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrNotFound), errors.Is(err, repository.ErrNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrConflict), errors.Is(err, repository.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, service.ErrAuthorization):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
