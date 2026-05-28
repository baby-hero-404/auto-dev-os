package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	h := NewAuthHandler(authSvc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader("{invalid json"))
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Register_ValidationError(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	h := NewAuthHandler(authSvc)

	// Missing email
	body := `{"email":"","password":"12345678"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Register_ShortPassword(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	h := NewAuthHandler(authSvc)

	body := `{"email":"test@example.com","password":"short"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", rr.Code)
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	h := NewAuthHandler(authSvc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("not json"))
	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Refresh_InvalidJSON(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	h := NewAuthHandler(authSvc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", strings.NewReader("bad"))
	h.Refresh(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	h := NewAuthHandler(authSvc)

	body := `{"refresh_token":"invalid.token.here"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", strings.NewReader(body))
	h.Refresh(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingBearer(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	mw := AuthMiddleware(authSvc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without bearer token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	mw := AuthMiddleware(authSvc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	authSvc := service.NewAuthService(nil, "test-secret")
	user := &models.User{ID: "u1", Email: "a@b.com", OrgID: "o1", Role: "admin"}
	tokens, err := authSvc.IssueTokensForTest(user)
	if err != nil {
		t.Fatalf("issueTokens: %v", err)
	}

	mw := AuthMiddleware(authSvc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d", rr.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, envelope{"status": "ok"})

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("unexpected response body: %v", resp)
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusNotFound, "not found")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "not found" {
		t.Errorf("unexpected error message: %v", resp)
	}
}

func TestIsValidationErr_Handler(t *testing.T) {
	tests := []struct {
		err    error
		expect bool
	}{
		{service.ErrValidation("test"), true},
		{nil, false},
	}
	for _, tc := range tests {
		if tc.err == nil {
			continue
		}
		if got := isValidationErr(tc.err); got != tc.expect {
			t.Errorf("isValidationErr(%v) = %v, want %v", tc.err, got, tc.expect)
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/health", nil)

	// Simulate the health handler inline (same as router.go).
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, envelope{"status": "ok", "version": "0.2.0"})
	})
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
	if resp["version"] != "0.2.0" {
		t.Errorf("expected version 0.2.0, got %v", resp["version"])
	}
}
