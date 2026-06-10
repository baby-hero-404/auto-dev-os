package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/go-chi/chi/v5"
)

type fakeProviderCredentialService struct {
	created models.CreateProviderCredentialInput
	updated models.UpdateProviderCredentialInput
	deleted string
	tested  string
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

type fakeVirtualKeyService struct {
	created models.CreateVirtualKeyInput
	updated models.UpdateVirtualKeyInput
	revoked string
}

func (s *fakeVirtualKeyService) Create(_ context.Context, orgID string, input models.CreateVirtualKeyInput) (*models.CreatedVirtualKeyResponse, error) {
	s.created = input
	return &models.CreatedVirtualKeyResponse{
		VirtualKeyResponse: models.VirtualKeyResponse{
			ID:        "vk-1",
			Name:      input.Name,
			KeyPrefix: "sk-aco-abcd",
			Status:    models.VirtualKeyStatusActive,
			CreatedAt: time.Unix(1, 0),
		},
		Key: "sk-aco-secret",
	}, nil
}

func (s *fakeVirtualKeyService) ListByOrg(_ context.Context, orgID string) ([]models.VirtualKeyResponse, error) {
	return []models.VirtualKeyResponse{{ID: "vk-1", Name: "default", KeyPrefix: "sk-aco-abcd", Status: models.VirtualKeyStatusActive}}, nil
}

func (s *fakeVirtualKeyService) GetByID(_ context.Context, id string) (*models.VirtualKeyResponse, error) {
	return &models.VirtualKeyResponse{ID: id, Name: "default", KeyPrefix: "sk-aco-abcd", Status: models.VirtualKeyStatusActive}, nil
}

func (s *fakeVirtualKeyService) Update(_ context.Context, id string, input models.UpdateVirtualKeyInput) (*models.VirtualKeyResponse, error) {
	s.updated = input
	return &models.VirtualKeyResponse{ID: id, Name: "updated", KeyPrefix: "sk-aco-abcd", Status: models.VirtualKeyStatusActive}, nil
}

func (s *fakeVirtualKeyService) Revoke(_ context.Context, id string) error {
	s.revoked = id
	return nil
}

type fakeModelRouteService struct {
	created models.CreateModelRouteInput
	updated models.UpdateModelRouteInput
	deleted string
}

func (s *fakeModelRouteService) Create(_ context.Context, orgID string, input models.CreateModelRouteInput) (*models.ModelRoute, error) {
	s.created = input
	return &models.ModelRoute{ID: "route-1", OrgID: orgID, Name: input.Name, RouteType: input.RouteType, Config: input.Config}, nil
}

func (s *fakeModelRouteService) ListByOrg(_ context.Context, orgID string) ([]models.ModelRoute, error) {
	return []models.ModelRoute{{ID: "route-1", OrgID: orgID, Name: "coding-default", RouteType: models.ModelRouteTypeCombo}}, nil
}

func (s *fakeModelRouteService) Update(_ context.Context, id string, input models.UpdateModelRouteInput) (*models.ModelRoute, error) {
	s.updated = input
	return &models.ModelRoute{ID: id, Name: "updated", RouteType: models.ModelRouteTypeCombo}, nil
}

func (s *fakeModelRouteService) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func TestProviderCredentialHandler_CreateMasksPlaintextKey(t *testing.T) {
	svc := &fakeProviderCredentialService{}
	h := NewProviderCredentialHandler(svc)
	rr := httptest.NewRecorder()
	req := requestWithURLParams("POST", "/", `{"provider":"openai","label":"primary","api_key":"sk-test-1234"}`, map[string]string{"orgID": "org-1"})

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.created.APIKey != "sk-test-1234" {
		t.Fatalf("service did not receive plaintext key")
	}
	var resp map[string]any
	decodeTestJSON(t, rr, &resp)
	if _, ok := resp["api_key"]; ok {
		t.Fatalf("response leaked api_key: %v", resp)
	}
	if resp["key_suffix"] != "1234" || resp["configured"] != true {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestProviderCredentialHandler_ListAndUpdateResponses(t *testing.T) {
	svc := &fakeProviderCredentialService{}
	h := NewProviderCredentialHandler(svc)

	listRR := httptest.NewRecorder()
	h.List(listRR, requestWithURLParams("GET", "/", "", map[string]string{"orgID": "org-1"}))
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRR.Code, listRR.Body.String())
	}
	var listResp []map[string]any
	decodeTestJSON(t, listRR, &listResp)
	if len(listResp) != 1 || listResp[0]["configured"] != true {
		t.Fatalf("unexpected list response: %v", listResp)
	}
	if _, ok := listResp[0]["api_key"]; ok {
		t.Fatalf("list response leaked api_key: %v", listResp)
	}

	updateRR := httptest.NewRecorder()
	h.Update(updateRR, requestWithURLParams("PUT", "/", `{"label":"updated"}`, map[string]string{"credentialID": "cred-1"}))
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d: %s", updateRR.Code, updateRR.Body.String())
	}
	if svc.updated.Label == nil || *svc.updated.Label != "updated" {
		t.Fatalf("service received wrong update: %+v", svc.updated)
	}
}

func TestProviderCredentialHandler_DeleteAndTestResponses(t *testing.T) {
	svc := &fakeProviderCredentialService{}
	h := NewProviderCredentialHandler(svc)

	deleteRR := httptest.NewRecorder()
	h.Delete(deleteRR, requestWithURLParams("DELETE", "/", "", map[string]string{"credentialID": "cred-1"}))
	if deleteRR.Code != http.StatusNoContent || svc.deleted != "cred-1" {
		t.Fatalf("delete = status %d id %q", deleteRR.Code, svc.deleted)
	}

	testRR := httptest.NewRecorder()
	h.Test(testRR, requestWithURLParams("POST", "/", "", map[string]string{"credentialID": "cred-1"}))
	if testRR.Code != http.StatusOK || svc.tested != "cred-1" {
		t.Fatalf("test = status %d id %q", testRR.Code, svc.tested)
	}
	var resp map[string]any
	decodeTestJSON(t, testRR, &resp)
	if resp["status"] != "configured" {
		t.Fatalf("unexpected test response: %v", resp)
	}
}

func TestVirtualKeyHandler_CreateReturnsRawKeyOnlyOnce(t *testing.T) {
	svc := &fakeVirtualKeyService{}
	h := NewVirtualKeyHandler(svc)
	rr := httptest.NewRecorder()
	req := requestWithURLParams("POST", "/", `{"name":"agent-default","budget_limit_usd":5}`, map[string]string{"orgID": "org-1"})

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if svc.created.Name != "agent-default" {
		t.Fatalf("service received wrong input: %+v", svc.created)
	}
	var resp map[string]any
	decodeTestJSON(t, rr, &resp)
	if resp["key"] != "sk-aco-secret" || resp["key_prefix"] != "sk-aco-abcd" {
		t.Fatalf("unexpected create response: %v", resp)
	}
	if _, ok := resp["key_hash"]; ok {
		t.Fatalf("response leaked key_hash: %v", resp)
	}
}

func TestVirtualKeyHandler_ListGetAndUpdateResponses(t *testing.T) {
	svc := &fakeVirtualKeyService{}
	h := NewVirtualKeyHandler(svc)

	listRR := httptest.NewRecorder()
	h.List(listRR, requestWithURLParams("GET", "/", "", map[string]string{"orgID": "org-1"}))
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRR.Code, listRR.Body.String())
	}
	var listResp []map[string]any
	decodeTestJSON(t, listRR, &listResp)
	if len(listResp) != 1 {
		t.Fatalf("unexpected list response: %v", listResp)
	}
	if _, ok := listResp[0]["key"]; ok {
		t.Fatalf("list response leaked raw key: %v", listResp)
	}

	getRR := httptest.NewRecorder()
	h.GetByID(getRR, requestWithURLParams("GET", "/", "", map[string]string{"virtualKeyID": "vk-1"}))
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d: %s", getRR.Code, getRR.Body.String())
	}

	updateRR := httptest.NewRecorder()
	h.Update(updateRR, requestWithURLParams("PUT", "/", `{"name":"updated"}`, map[string]string{"virtualKeyID": "vk-1"}))
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d: %s", updateRR.Code, updateRR.Body.String())
	}
	if svc.updated.Name == nil || *svc.updated.Name != "updated" {
		t.Fatalf("service received wrong update: %+v", svc.updated)
	}
}

func TestVirtualKeyHandler_RevokeResponse(t *testing.T) {
	svc := &fakeVirtualKeyService{}
	h := NewVirtualKeyHandler(svc)
	rr := httptest.NewRecorder()

	h.Revoke(rr, requestWithURLParams("DELETE", "/", "", map[string]string{"virtualKeyID": "vk-1"}))

	if rr.Code != http.StatusNoContent || svc.revoked != "vk-1" {
		t.Fatalf("revoke = status %d id %q", rr.Code, svc.revoked)
	}
}

func TestModelRouteHandler_CRUDResponses(t *testing.T) {
	svc := &fakeModelRouteService{}
	h := NewModelRouteHandler(svc)

	createRR := httptest.NewRecorder()
	h.Create(createRR, requestWithURLParams("POST", "/", `{"name":"coding-default","route_type":"combo","config":[{"provider":"openai","model":"gpt-4o","priority":0}]}`, map[string]string{"orgID": "org-1"}))
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createRR.Code, createRR.Body.String())
	}
	if svc.created.Name != "coding-default" {
		t.Fatalf("service received wrong create: %+v", svc.created)
	}

	listRR := httptest.NewRecorder()
	h.List(listRR, requestWithURLParams("GET", "/", "", map[string]string{"orgID": "org-1"}))
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRR.Code, listRR.Body.String())
	}

	updateRR := httptest.NewRecorder()
	h.Update(updateRR, requestWithURLParams("PUT", "/", `{"name":"updated"}`, map[string]string{"routeID": "route-1"}))
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d: %s", updateRR.Code, updateRR.Body.String())
	}
	if svc.updated.Name == nil || *svc.updated.Name != "updated" {
		t.Fatalf("service received wrong update: %+v", svc.updated)
	}

	deleteRR := httptest.NewRecorder()
	h.Delete(deleteRR, requestWithURLParams("DELETE", "/", "", map[string]string{"routeID": "route-1"}))
	if deleteRR.Code != http.StatusNoContent || svc.deleted != "route-1" {
		t.Fatalf("delete = status %d id %q", deleteRR.Code, svc.deleted)
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
