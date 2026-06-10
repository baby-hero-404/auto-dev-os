package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

var (
	ErrBudgetExhausted   = errors.New("virtual key budget exhausted")
	ErrVirtualKeyInvalid = errors.New("virtual key invalid")
)

type VirtualKeyService struct {
	repo  *repository.VirtualKeyRepo
	audit *AuditService
}

func NewVirtualKeyService(repo *repository.VirtualKeyRepo) *VirtualKeyService {
	return &VirtualKeyService{repo: repo}
}

func (s *VirtualKeyService) WithAudit(audit *AuditService) *VirtualKeyService {
	s.audit = audit
	return s
}

func (s *VirtualKeyService) Create(ctx context.Context, orgID string, input models.CreateVirtualKeyInput) (*models.CreatedVirtualKeyResponse, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	key, err := generateVirtualKey()
	if err != nil {
		return nil, err
	}
	hash := HashVirtualKey(key)
	prefix := "sk-aco-" + key[len(key)-4:]
	model, err := s.repo.Create(ctx, orgID, input, hash, prefix)
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, models.AuditActionVirtualKeyCreated, model, map[string]any{
		"name":       model.Name,
		"key_prefix": model.KeyPrefix,
		"project_id": model.ProjectID,
		"agent_id":   model.AgentID,
	})
	resp := models.CreatedVirtualKeyResponse{VirtualKeyResponse: model.ToResponse(), Key: key}
	return &resp, nil
}

func (s *VirtualKeyService) Validate(ctx context.Context, rawKey string, estimatedCost float64) (*models.VirtualKey, error) {
	if rawKey == "" {
		return nil, ErrVirtualKeyInvalid
	}
	key, err := s.repo.FindByHash(ctx, HashVirtualKey(rawKey))
	if err != nil {
		return nil, err
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, ErrVirtualKeyInvalid
	}
	if key.BudgetLimitUSD != nil && key.BudgetUsedUSD+estimatedCost > *key.BudgetLimitUSD {
		return nil, ErrBudgetExhausted
	}
	s.recordAudit(ctx, models.AuditActionVirtualKeyUsed, key, map[string]any{
		"name":           key.Name,
		"key_prefix":     key.KeyPrefix,
		"estimated_cost": estimatedCost,
	})
	return key, nil
}

func (s *VirtualKeyService) ListByOrg(ctx context.Context, orgID string) ([]models.VirtualKeyResponse, error) {
	keys, err := s.repo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]models.VirtualKeyResponse, 0, len(keys))
	for _, key := range keys {
		out = append(out, key.ToResponse())
	}
	return out, nil
}

func (s *VirtualKeyService) GetByID(ctx context.Context, id string) (*models.VirtualKeyResponse, error) {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := key.ToResponse()
	return &resp, nil
}

func (s *VirtualKeyService) Update(ctx context.Context, id string, input models.UpdateVirtualKeyInput) (*models.VirtualKeyResponse, error) {
	key, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, models.AuditActionVirtualKeyUpdated, key, map[string]any{
		"name":       key.Name,
		"key_prefix": key.KeyPrefix,
		"status":     key.Status,
	})
	resp := key.ToResponse()
	return &resp, nil
}

func (s *VirtualKeyService) RecordUsage(ctx context.Context, virtualKeyID string, costUSD float64) error {
	if costUSD <= 0 {
		slog.Debug("skip virtual key usage record with non-positive cost", "virtual_key_id", virtualKeyID, "cost_usd", costUSD)
		return nil
	}
	return s.repo.IncrementBudgetUsed(ctx, virtualKeyID, costUSD)
}

func (s *VirtualKeyService) Revoke(ctx context.Context, id string) error {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Revoke(ctx, id); err != nil {
		return err
	}
	key.Status = models.VirtualKeyStatusRevoked
	s.recordAudit(ctx, models.AuditActionVirtualKeyRevoked, key, map[string]any{
		"name":       key.Name,
		"key_prefix": key.KeyPrefix,
	})
	return nil
}

func (s *VirtualKeyService) recordAudit(ctx context.Context, action string, key *models.VirtualKey, details map[string]any) {
	if s.audit == nil || key == nil {
		return
	}
	s.audit.RecordAction(ctx, action, "virtual_key", key.ID,
		WithOrgID(key.OrgID),
		WithDetails(details),
	)
}

func HashVirtualKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateVirtualKey() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, 32)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		buf[i] = alphabet[n.Int64()]
	}
	return "sk-aco-" + string(buf), nil
}
