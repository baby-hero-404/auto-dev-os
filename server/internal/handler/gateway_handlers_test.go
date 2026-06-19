package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type fakeProviderCredentialService struct {
	created models.CreateProviderCredentialInput
	updated models.UpdateProviderCredentialInput
	deleted string
	tested  string
	testRaw models.TestProviderCredentialInput
}

func (s *fakeProviderCredentialService) Create(_ context.Context, orgID string, input models.CreateProviderCredentialInput) (*models.ProviderCredentialResponse, error) {
	s.created = input
	return &models.ProviderCredentialResponse{
		ID:         "cred-1",
		Provider:   input.Provider,
		Label:      input.Label,
		Status:     models.ProviderCredentialStatusActive,
		Configured: true,
		KeySuffix:  "1234",
	}, nil
}

func (s *fakeProviderCredentialService) ListByOrg(_ context.Context, orgID string) ([]models.ProviderCredentialResponse, error) {
	return []models.ProviderCredentialResponse{{
		ID:         "cred-1",
		Provider:   "openai",
		Label:      "primary",
		Status:     models.ProviderCredentialStatusActive,
		Configured: true,
	}}, nil
}

func (s *fakeProviderCredentialService) Update(_ context.Context, id string, input models.UpdateProviderCredentialInput) (*models.ProviderCredentialResponse, error) {
	s.updated = input
	return &models.ProviderCredentialResponse{ID: id, Provider: "openai", Label: "updated", Status: models.ProviderCredentialStatusActive, Configured: true}, nil
}

func (s *fakeProviderCredentialService) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func (s *fakeProviderCredentialService) TestConnection(_ context.Context, id string) error {
	s.tested = id
	return nil
}

func (s *fakeProviderCredentialService) TestConnectionInput(_ context.Context, input models.TestProviderCredentialInput) error {
	s.testRaw = input
	return nil
}

type fakeProviderModelService struct {
	created models.CreateProviderModelInput
	updated models.UpdateProviderModelInput
	deleted string
}

func (s *fakeProviderModelService) Create(ctx context.Context, orgID string, input models.CreateProviderModelInput) (*models.ProviderModel, error) {
	s.created = input
	return &models.ProviderModel{ID: "pm-1", OrgID: orgID, Provider: input.Provider, ModelName: input.ModelName, LevelGroup: input.LevelGroup, Priority: input.Priority, IsActive: true}, nil
}

func (s *fakeProviderModelService) ListByOrg(ctx context.Context, orgID string, filter models.ProviderModelFilter) ([]models.ProviderModel, error) {
	return []models.ProviderModel{{ID: "pm-1", OrgID: orgID, Provider: "openai", ModelName: "gpt-4o", LevelGroup: "balanced", Priority: 0, IsActive: true}}, nil
}

func (s *fakeProviderModelService) Update(ctx context.Context, orgID string, id string, input models.UpdateProviderModelInput) (*models.ProviderModel, error) {
	s.updated = input
	return &models.ProviderModel{ID: id, OrgID: orgID, Provider: "openai", ModelName: "gpt-4o", LevelGroup: "balanced", Priority: 0, IsActive: true}, nil
}

func (s *fakeProviderModelService) Delete(ctx context.Context, orgID string, id string) error {
	s.deleted = id
	return nil
}

func TestProviderModelHandler_CRUDResponses(t *testing.T) {
	svc := &fakeProviderModelService{}
	h := NewProviderModelHandler(svc)

	createRR := httptest.NewRecorder()
	h.Create(createRR, requestWithURLParams("POST", "/", `{"provider":"openai","model_name":"gpt-4o","level_group":"balanced","priority":0}`, map[string]string{"orgID": "org-1"}))
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRR.Code, createRR.Body.String())
	}
	if svc.created.ModelName != "gpt-4o" {
		t.Fatalf("service received wrong create: %+v", svc.created)
	}

	listRR := httptest.NewRecorder()
	h.List(listRR, requestWithURLParams("GET", "/", "", map[string]string{"orgID": "org-1"}))
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRR.Code, listRR.Body.String())
	}

	updateRR := httptest.NewRecorder()
	h.Update(updateRR, requestWithURLParams("PUT", "/", `{"priority":1}`, map[string]string{"modelID": "pm-1", "orgID": "org-1"}))
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d: %s", updateRR.Code, updateRR.Body.String())
	}
	if svc.updated.Priority == nil || *svc.updated.Priority != 1 {
		t.Fatalf("service received wrong update: %+v", svc.updated)
	}

	deleteRR := httptest.NewRecorder()
	h.Delete(deleteRR, requestWithURLParams("DELETE", "/", "", map[string]string{"modelID": "pm-1", "orgID": "org-1"}))
	if deleteRR.Code != http.StatusNoContent || svc.deleted != "pm-1" {
		t.Fatalf("delete = status %d id %q", deleteRR.Code, svc.deleted)
	}
}

func TestProviderModelHandler_ListValidation(t *testing.T) {
	svc := &fakeProviderModelService{}
	h := NewProviderModelHandler(svc)

	// Test invalid level_group
	invalidRR := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/?level_group=gpt-4o", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	h.List(invalidRR, req)
	if invalidRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid level_group, got %d", invalidRR.Code)
	}

	// Test valid level_group
	validRR := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/?level_group=balanced", nil)
	routeCtx2 := chi.NewRouteContext()
	routeCtx2.URLParams.Add("orgID", "org-1")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, routeCtx2))
	h.List(validRR, req2)
	if validRR.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid level_group, got %d", validRR.Code)
	}
}

func requestWithURLParams(method, target, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	for k, v := range params {
		routeCtx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func decodeTestJSON(t *testing.T, rr *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rr.Body.String())
	}
}
