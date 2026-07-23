package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type AttestationHandler struct {
	svc AttestationService
}

func NewAttestationHandler(svc AttestationService) *AttestationHandler {
	return &AttestationHandler{svc: svc}
}

// GetByCommit implements GET /attestations/{commit} (REQ-004): returns the
// envelope plus a verify result. A commit with no attestation is 404.
func (h *AttestationHandler) GetByCommit(w http.ResponseWriter, r *http.Request) {
	commit := chi.URLParam(r, "commit")
	if commit == "" {
		writeError(w, http.StatusBadRequest, "commit is required")
		return
	}
	result, err := h.svc.VerifyByCommitHash(r.Context(), commit)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "no attestation found for commit")
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"envelope": result.Envelope,
		"verified": result.Verified,
		"key_id":   result.KeyID,
	})
}

// ListByTask implements GET /tasks/{taskID}/attestations for the Audit panel
// (REQ-005).
func (h *AttestationHandler) ListByTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "taskID is required")
		return
	}
	attestations, err := h.svc.ListByTaskID(r.Context(), taskID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attestations)
}

// Keys implements GET /attestations/keys (REQ-006): the deployment's full
// JWKS-format keyset (active + retired), for offline verification.
func (h *AttestationHandler) Keys(w http.ResponseWriter, r *http.Request) {
	jwks, err := h.svc.JWKS(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, jwks)
}
