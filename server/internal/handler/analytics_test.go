package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type fakeAnalyticsService struct {
	orgID     string
	projectID string
	since     time.Time
}

func (s *fakeAnalyticsService) TokenUsage(_ context.Context, orgID string, projectID string, since time.Time) ([]models.TokenUsageSummary, error) {
	s.orgID = orgID
	s.projectID = projectID
	s.since = since
	return []models.TokenUsageSummary{}, nil
}

func TestAnalyticsHandler_TokenUsageRequiresOrgID(t *testing.T) {
	h := NewAnalyticsHandler(&fakeAnalyticsService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/token-usage", nil)
	h.TokenUsage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when org_id is missing, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "org_id is required" {
		t.Fatalf("unexpected error response: %v", resp)
	}
}

func TestAnalyticsHandler_TokenUsageUsesQueryOrgID(t *testing.T) {
	svc := &fakeAnalyticsService{}
	h := NewAnalyticsHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/analytics/token-usage?org_id=org-1&project_id=project-1", nil)
	routeCtx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	h.TokenUsage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.orgID != "org-1" || svc.projectID != "project-1" {
		t.Fatalf("service received wrong params: org=%q project=%q", svc.orgID, svc.projectID)
	}
}
