package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// AuditHandler handles the audit log API endpoints.
type AuditHandler struct {
	svc AuditService
}

func NewAuditHandler(svc AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List returns audit logs matching query filters.
// GET /api/v1/audit/logs?org_id=&action=&entity_type=&task_id=&since=&limit=
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := models.AuditLogFilter{
		OrgID:      q.Get("org_id"),
		UserID:     q.Get("user_id"),
		AgentID:    q.Get("agent_id"),
		TaskID:     q.Get("task_id"),
		Action:     q.Get("action"),
		EntityType: q.Get("entity_type"),
	}

	if sinceRaw := q.Get("since"); sinceRaw != "" {
		if t, err := time.Parse(time.RFC3339, sinceRaw); err == nil {
			filter.Since = t
		}
	}
	if daysRaw := q.Get("days"); daysRaw != "" {
		if d, err := strconv.Atoi(daysRaw); err == nil && d > 0 {
			filter.Since = time.Now().AddDate(0, 0, -d)
		}
	}
	if limitRaw := q.Get("limit"); limitRaw != "" {
		if l, err := strconv.Atoi(limitRaw); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	logs, err := h.svc.List(r.Context(), filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// Summary returns audit log counts grouped by action.
// GET /api/v1/audit/summary?org_id=...
func (h *AuditHandler) Summary(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	counts, err := h.svc.CountByAction(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
