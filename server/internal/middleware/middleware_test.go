package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(5, 5, time.Second)

	// First 5 requests should pass (burst = 5).
	for i := 0; i < 5; i++ {
		if !rl.Allow("user1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	// 6th request should be rejected.
	if rl.Allow("user1") {
		t.Error("6th request should be rate-limited")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(2, 2, time.Second)

	rl.Allow("user1")
	rl.Allow("user1")
	if rl.Allow("user1") {
		t.Error("user1 should be rate-limited")
	}
	// user2 should be independent
	if !rl.Allow("user2") {
		t.Error("user2 should not be rate-limited")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(10, 10, 50*time.Millisecond)

	// Exhaust tokens.
	for i := 0; i < 10; i++ {
		rl.Allow("user1")
	}
	if rl.Allow("user1") {
		t.Error("should be rate-limited after exhaustion")
	}
	// Wait for refill.
	time.Sleep(100 * time.Millisecond)
	if !rl.Allow("user1") {
		t.Error("should be allowed after refill interval")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(2, 2, time.Second)
	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 2 requests pass.
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
	// 3rd request should be rejected.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), authClaimsKey, &tokenClaims{Subject: "u1", Role: "admin"})
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for admin role, got %d", rr.Code)
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), authClaimsKey, &tokenClaims{Subject: "u1", Role: "member"})
	handler.ServeHTTP(rr, req.WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for member role, got %d", rr.Code)
	}
}

func TestRequireRole_NoClaims(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without claims, got %d", rr.Code)
	}
}
