package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
)

type AnalyticsHandler struct {
	svc *service.AnalyticsService
}

func NewAnalyticsHandler(svc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

func (h *AnalyticsHandler) TokenUsage(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	since := time.Time{}
	if daysRaw := r.URL.Query().Get("days"); daysRaw != "" {
		days, err := strconv.Atoi(daysRaw)
		if err != nil || days < 1 {
			writeError(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		since = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	}
	usage, err := h.svc.TokenUsage(r.Context(), projectID, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, usage)
}
