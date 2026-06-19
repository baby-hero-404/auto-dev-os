package handler

import (
	"net/http"
	"strconv"
)

// AnalyticsDashboardHandler handles the Phase 5 analytics dashboard endpoints.
type AnalyticsDashboardHandler struct {
	svc AnalyticsDashboardService
}

func NewAnalyticsDashboardHandler(svc AnalyticsDashboardService) *AnalyticsDashboardHandler {
	return &AnalyticsDashboardHandler{svc: svc}
}

// Overview returns high-level platform statistics.
// GET /api/v1/analytics/overview?org_id=...
func (h *AnalyticsDashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	stats, err := h.svc.Overview(r.Context(), orgID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// AgentPerformance returns per-agent performance metrics.
// GET /api/v1/analytics/agents?org_id=...&project_id=...
func (h *AnalyticsDashboardHandler) AgentPerformance(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	projectID := r.URL.Query().Get("project_id")
	stats, err := h.svc.AgentPerformance(r.Context(), orgID, projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// TaskAnalytics returns task throughput over time and status distribution.
// GET /api/v1/analytics/tasks?org_id=...&project_id=...&days=30
func (h *AnalyticsDashboardHandler) TaskAnalytics(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	projectID := r.URL.Query().Get("project_id")
	days := 30
	if daysRaw := r.URL.Query().Get("days"); daysRaw != "" {
		if d, err := strconv.Atoi(daysRaw); err == nil && d > 0 {
			days = d
		}
	}
	analytics, err := h.svc.TaskAnalytics(r.Context(), orgID, projectID, days)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, analytics)
}

// GatewayUsage returns daily gateway usage aggregates.
// GET /api/v1/analytics/gateway-usage?org_id=...&project_id=...&days=30
func (h *AnalyticsDashboardHandler) GatewayUsage(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	projectID := r.URL.Query().Get("project_id")
	days := 30
	if daysRaw := r.URL.Query().Get("days"); daysRaw != "" {
		if d, err := strconv.Atoi(daysRaw); err == nil && d > 0 {
			days = d
		}
	}
	usage, err := h.svc.GatewayUsage(r.Context(), orgID, projectID, days)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

// WorkflowAnalytics returns workflow completion rates and step durations.
// GET /api/v1/analytics/workflows?org_id=...&project_id=...
func (h *AnalyticsDashboardHandler) WorkflowAnalytics(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	projectID := r.URL.Query().Get("project_id")
	analytics, err := h.svc.WorkflowAnalytics(r.Context(), orgID, projectID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, analytics)
}

// RecentFailures returns latest failed tasks with workflow error context.
// GET /api/v1/analytics/failures?org_id=...&project_id=...&limit=5
func (h *AnalyticsDashboardHandler) RecentFailures(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	projectID := r.URL.Query().Get("project_id")
	limit := 5
	if limitRaw := r.URL.Query().Get("limit"); limitRaw != "" {
		if parsed, err := strconv.Atoi(limitRaw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	failures, err := h.svc.RecentFailures(r.Context(), orgID, projectID, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, failures)
}
