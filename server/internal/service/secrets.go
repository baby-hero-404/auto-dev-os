package service

import (
	"context"
	"crypto/cipher"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type SecretService struct {
	repo *repository.SecretRepo
	aead cipher.AEAD
}

func NewSecretService(repo *repository.SecretRepo, keyMaterial string) (*SecretService, error) {
	secretCipher, err := NewSecretCipher(keyMaterial)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	return &SecretService{repo: repo, aead: secretCipher.aead}, nil
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

func (s *SecretService) encrypt(plain string) (string, error) {
	return (&SecretCipher{aead: s.aead}).Encrypt(plain)
}
