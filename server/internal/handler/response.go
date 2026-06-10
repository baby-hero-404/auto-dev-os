package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
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

func writeServiceError(w http.ResponseWriter, err error) {
	status := statusForError(err)
	msg := err.Error()
	if status == http.StatusNotFound {
		msg = "not found"
	}
	writeError(w, status, msg)
}

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

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
