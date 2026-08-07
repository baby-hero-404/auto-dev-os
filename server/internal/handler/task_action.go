package handler

import (
	"net/http"

	"github.com/auto-code-os/auto-code-os/server/internal/middleware"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
)

type TaskActionHandler struct {
	svc *service.TaskActionService
}

func NewTaskActionHandler(svc *service.TaskActionService) *TaskActionHandler {
	return &TaskActionHandler{svc: svc}
}

// Dispatch handles POST /tasks/{taskID}/actions — the single action-dispatch
// endpoint for docs/openspecs/status-driven-agent-workspace (specs.md:
// single-approval-gate + idempotent action dispatch).
func (h *TaskActionHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	var input service.TaskActionInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	t, err := h.svc.Dispatch(r.Context(), taskID, claims.Role, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
