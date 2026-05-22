package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
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

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// isValidationErr checks if an error is a service-layer validation error.
func isValidationErr(err error) bool {
	return strings.HasPrefix(err.Error(), "validation:")
}
