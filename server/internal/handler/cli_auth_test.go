package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/go-chi/chi/v5"
)

func TestMintWSTicket_Success(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	claims := &service.TokenClaims{Subject: "user-1", OrgID: "org-1"}
	req := httptest.NewRequest("POST", "/organizations/org-1/cli-auth/ws-ticket", strings.NewReader(`{"provider":"claude"}`))
	req = req.WithContext(context.WithValue(req.Context(), authClaimsKey, claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.MintWSTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad response body: %v", err)
	}
	if resp.Ticket == "" || resp.ExpiresIn != 20 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestMintWSTicket_OrgMismatchForbidden(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	claims := &service.TokenClaims{Subject: "user-1", OrgID: "org-1"}
	req := httptest.NewRequest("POST", "/organizations/org-2/cli-auth/ws-ticket", strings.NewReader(`{"provider":"claude"}`))
	req = req.WithContext(context.WithValue(req.Context(), authClaimsKey, claims))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-2")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.MintWSTicket(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestTerminal_MissingTicket_Unauthorized(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	req := httptest.NewRequest("GET", "/organizations/org-1/cli-auth/terminal", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Terminal(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestTerminal_InvalidTicket_Unauthorized(t *testing.T) {
	h := NewCLIAuthHandler(nil)

	req := httptest.NewRequest("GET", "/organizations/org-1/cli-auth/terminal?ticket=does-not-exist", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Terminal(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestTerminal_TicketFromDifferentOrg_Unauthorized(t *testing.T) {
	h := NewCLIAuthHandler(nil)
	token, _ := h.tickets.Mint("user-1", "org-A", "claude")

	req := httptest.NewRequest("GET", "/organizations/org-B/cli-auth/terminal?ticket="+token, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orgID", "org-B")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.Terminal(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for cross-org ticket, got %d", rr.Code)
	}
}
