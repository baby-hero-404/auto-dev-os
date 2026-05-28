package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type SecretService struct {
	repo *repository.SecretRepo
	aead cipher.AEAD
}

func NewSecretService(repo *repository.SecretRepo, keyMaterial string) (*SecretService, error) {
	if strings.TrimSpace(keyMaterial) == "" {
		keyMaterial = "auto-code-os-dev-secret"
	}
	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret aead: %w", err)
	}
	return &SecretService{repo: repo, aead: aead}, nil
}

func (s *SecretService) Upsert(ctx context.Context, projectID string, input models.CreateSecretInput) (*models.Secret, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrValidation("secret name is required")
	}
	if input.Value == "" {
		return nil, ErrValidation("secret value is required")
	}
	encrypted, err := s.encrypt(input.Value)
	if err != nil {
		return nil, err
	}
	input.Value = encrypted
	return s.repo.Upsert(ctx, projectID, input)
}

func (s *SecretService) RuntimeEnv(ctx context.Context, projectID string) (map[string]string, error) {
	secrets, err := s.repo.ListByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(secrets))
	for _, secret := range secrets {
		value, err := s.decrypt(secret.Value)
		if err != nil {
			return nil, err
		}
		env[secret.Name] = value
	}
	return env, nil
}

func (s *SecretService) encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	ciphertext := s.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *SecretService) decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	nonceSize := s.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("secret ciphertext too short")
	}
	plain, err := s.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plain), nil
}
