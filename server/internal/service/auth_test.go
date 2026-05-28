package service

import (
	"context"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestAuthService_IssueAndVerifyToken(t *testing.T) {
	svc := NewAuthService(nil, "test-secret-key-32chars-minimum!")

	user := &models.User{
		ID:    "user-123",
		Email: "test@example.com",
		OrgID: "org-456",
		Role:  models.UserRoleAdmin,
	}

	tokens, err := svc.issueTokens(user)
	if err != nil {
		t.Fatalf("issueTokens: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("access token is empty")
	}
	if tokens.RefreshToken == "" {
		t.Error("refresh token is empty")
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("expected Bearer token type, got %q", tokens.TokenType)
	}

	// Verify access token
	claims, err := svc.VerifyToken(tokens.AccessToken, "access")
	if err != nil {
		t.Fatalf("VerifyToken access: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("expected subject user-123, got %q", claims.Subject)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", claims.Email)
	}
	if claims.OrgID != "org-456" {
		t.Errorf("expected org_id org-456, got %q", claims.OrgID)
	}
	if claims.Role != models.UserRoleAdmin {
		t.Errorf("expected role admin, got %q", claims.Role)
	}

	// Verify refresh token
	refreshClaims, err := svc.VerifyToken(tokens.RefreshToken, "refresh")
	if err != nil {
		t.Fatalf("VerifyToken refresh: %v", err)
	}
	if refreshClaims.Subject != "user-123" {
		t.Errorf("refresh token subject mismatch")
	}
}

func TestAuthService_VerifyToken_WrongType(t *testing.T) {
	svc := NewAuthService(nil, "test-secret")

	user := &models.User{ID: "u1", Email: "a@b.com", OrgID: "o1", Role: "admin"}
	tokens, _ := svc.issueTokens(user)

	// Using access token as refresh should fail
	_, err := svc.VerifyToken(tokens.AccessToken, "refresh")
	if err == nil {
		t.Error("expected error when verifying access token as refresh type")
	}
}

func TestAuthService_VerifyToken_InvalidSignature(t *testing.T) {
	svc1 := NewAuthService(nil, "secret-1")
	svc2 := NewAuthService(nil, "secret-2")

	user := &models.User{ID: "u1", Email: "a@b.com", OrgID: "o1", Role: "admin"}
	tokens, _ := svc1.issueTokens(user)

	_, err := svc2.VerifyToken(tokens.AccessToken, "access")
	if err == nil {
		t.Error("expected error when verifying with wrong secret")
	}
}

func TestAuthService_VerifyToken_MalformedToken(t *testing.T) {
	svc := NewAuthService(nil, "test-secret")

	tests := []string{
		"",
		"not.a.valid.token",
		"only-one-part",
		"two.parts",
	}
	for _, token := range tests {
		_, err := svc.VerifyToken(token, "access")
		if err == nil {
			t.Errorf("expected error for token %q", token)
		}
	}
}

func TestAuthService_Register_Validation(t *testing.T) {
	svc := NewAuthService(nil, "test-secret")

	// Empty email
	_, err := svc.Register(context.Background(), models.RegisterInput{Email: "", Password: "12345678"})
	if err == nil {
		t.Error("expected error for empty email")
	}

	// Short password
	_, err = svc.Register(context.Background(), models.RegisterInput{Email: "a@b.com", Password: "short"})
	if err == nil {
		t.Error("expected error for short password")
	}
}

func TestAuthService_DefaultSecret(t *testing.T) {
	svc := NewAuthService(nil, "")
	user := &models.User{ID: "u1", Email: "a@b.com", OrgID: "o1", Role: "admin"}
	tokens, err := svc.issueTokens(user)
	if err != nil {
		t.Fatalf("should work with default secret: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("access token should not be empty")
	}
}

func TestAuthService_TokenExpiry(t *testing.T) {
	svc := NewAuthService(nil, "test-secret")
	user := &models.User{ID: "u1", Email: "a@b.com", OrgID: "o1", Role: "admin"}

	tokens, _ := svc.issueTokens(user)
	claims, _ := svc.VerifyToken(tokens.AccessToken, "access")

	// Token should expire in the future
	if claims.Expires <= time.Now().Unix() {
		t.Error("token should not be expired immediately after creation")
	}
}
