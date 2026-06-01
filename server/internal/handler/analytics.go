package handler

import (
	"net/http"
	"strconv"
	"time"
)

type AnalyticsHandler struct {
	svc AnalyticsService
}

func NewAnalyticsHandler(svc AnalyticsService) *AnalyticsHandler {
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
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usage)
}
