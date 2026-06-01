package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

type AuthService struct {
	repo      *repository.AuthRepo
	jwtSecret []byte
}

type TokenClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	OrgID   string `json:"org_id"`
	Role    string `json:"role"`
	Type    string `json:"typ"`
	Expires int64  `json:"exp"`
}

func NewAuthService(repo *repository.AuthRepo, jwtSecret string) *AuthService {
	if jwtSecret == "" {
		jwtSecret = "dev-only-change-me"
	}
	return &AuthService{repo: repo, jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) Register(ctx context.Context, input models.RegisterInput) (*models.AuthResponse, error) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if input.Email == "" {
		return nil, ErrValidation("email is required")
	}
	if len(input.Password) < 8 {
		return nil, ErrValidation("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.CreateUserWithOrganization(ctx, input, string(hash))
	if err != nil {
		return nil, err
	}
	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{User: user, Tokens: tokens}, nil
}

func (s *AuthService) Login(ctx context.Context, input models.LoginInput) (*models.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrAuthorizationf("invalid email or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrAuthorizationf("invalid email or password")
	}
	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{User: user, Tokens: tokens}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	claims, err := s.VerifyToken(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetUserByID(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}
	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{User: user, Tokens: tokens}, nil
}

func (s *AuthService) VerifyToken(token, expectedType string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrAuthorizationf("invalid token")
	}
	signed := parts[0] + "." + parts[1]
	want := signJWTPart(signed, s.jwtSecret)
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return nil, ErrAuthorizationf("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrAuthorizationf("invalid token payload")
	}
	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrAuthorizationf("invalid token claims")
	}
	if claims.Type != expectedType {
		return nil, ErrAuthorizationf("invalid token type")
	}
	if time.Now().Unix() >= claims.Expires {
		return nil, ErrAuthorizationf("token expired")
	}
	return &claims, nil
}

func (s *AuthService) issueTokens(user *models.User) (models.AuthTokens, error) {
	access, err := s.issueToken(user, "access", accessTokenTTL)
	if err != nil {
		return models.AuthTokens{}, err
	}
	refresh, err := s.issueToken(user, "refresh", refreshTokenTTL)
	if err != nil {
		return models.AuthTokens{}, err
	}
	return models.AuthTokens{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
	}, nil
}

func (s *AuthService) issueToken(user *models.User, tokenType string, ttl time.Duration) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(TokenClaims{
		Subject: user.ID,
		Email:   user.Email,
		OrgID:   user.OrgID,
		Role:    user.Role,
		Type:    tokenType,
		Expires: time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(header)
	p := base64.RawURLEncoding.EncodeToString(claims)
	signed := h + "." + p
	return signed + "." + signJWTPart(signed, s.jwtSecret), nil
}

func signJWTPart(value string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
