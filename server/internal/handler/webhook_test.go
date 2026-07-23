package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyGitHubSignature(t *testing.T) {
	secret := "topsecret"
	body := []byte(`{"action":"opened"}`)

	if !verifyGitHubSignature(secret, body, signBody(secret, body)) {
		t.Error("expected valid signature to verify")
	}
	if verifyGitHubSignature(secret, body, signBody("wrong-secret", body)) {
		t.Error("expected signature with wrong secret to fail")
	}
	if verifyGitHubSignature(secret, []byte(`{"action":"tampered"}`), signBody(secret, body)) {
		t.Error("expected signature over different body to fail")
	}
	if verifyGitHubSignature(secret, body, "") {
		t.Error("expected missing signature header to fail")
	}
	if verifyGitHubSignature(secret, body, hex.EncodeToString([]byte("no-prefix"))) {
		t.Error("expected signature without sha256= prefix to fail")
	}
}

func TestWebhookGitHub_RejectsUnsignedWhenSecretSet(t *testing.T) {
	t.Setenv("WEBHOOK_SECRET", "topsecret")
	h := NewWebhookHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(`{"action":"opened"}`))
	req.Header.Set("X-GitHub-Event", "issues")
	rec := httptest.NewRecorder()
	h.GitHub(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unsigned payload, got %d", rec.Code)
	}
}

func TestWebhookGitHub_AcceptsSignedPayload(t *testing.T) {
	t.Setenv("WEBHOOK_SECRET", "topsecret")
	h := NewWebhookHandler(nil, nil)

	body := `{"action":"opened"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signBody("topsecret", []byte(body)))
	rec := httptest.NewRecorder()
	h.GitHub(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected signed payload to pass auth, got 401: %s", rec.Body.String())
	}
}
