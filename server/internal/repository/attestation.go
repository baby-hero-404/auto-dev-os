package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// AttestationRepo persists signed per-commit attestations.
type AttestationRepo struct{ db *gorm.DB }

func NewAttestationRepo(db *gorm.DB) *AttestationRepo {
	return &AttestationRepo{db: db}
}

func (r *AttestationRepo) Create(ctx context.Context, input models.CreateAttestationInput) (*models.Attestation, error) {
	a := &models.Attestation{
		TaskID:         input.TaskID,
		JobID:          input.JobID,
		CommitHash:     input.CommitHash,
		KeyID:          input.KeyID,
		CodedBy:        input.CodedBy,
		ReviewedBy:     input.ReviewedBy,
		PromptHash:     input.PromptHash,
		PolicySnapshot: input.PolicySnapshot,
		Envelope:       input.Envelope,
	}
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("create attestation: %w", err)
	}
	return a, nil
}

func (r *AttestationRepo) GetByCommitHash(ctx context.Context, commitHash string) (*models.Attestation, error) {
	var a models.Attestation
	if err := r.db.WithContext(ctx).Where("commit_hash = ?", commitHash).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AttestationRepo) ListByTaskID(ctx context.Context, taskID string) ([]models.Attestation, error) {
	var out []models.Attestation
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list attestations: %w", err)
	}
	return out, nil
}

// AttestationKeyRepo persists the per-deployment Ed25519 signing keyset.
type AttestationKeyRepo struct{ db *gorm.DB }

func NewAttestationKeyRepo(db *gorm.DB) *AttestationKeyRepo {
	return &AttestationKeyRepo{db: db}
}

func (r *AttestationKeyRepo) Create(ctx context.Context, key *models.AttestationKey) error {
	if err := r.db.WithContext(ctx).Create(key).Error; err != nil {
		return fmt.Errorf("create attestation key: %w", err)
	}
	return nil
}

func (r *AttestationKeyRepo) GetActive(ctx context.Context) (*models.AttestationKey, error) {
	var k models.AttestationKey
	if err := r.db.WithContext(ctx).Where("status = ?", models.AttestationKeyActive).Order("created_at DESC").First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *AttestationKeyRepo) GetByKeyID(ctx context.Context, keyID string) (*models.AttestationKey, error) {
	var k models.AttestationKey
	if err := r.db.WithContext(ctx).Where("key_id = ?", keyID).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *AttestationKeyRepo) ListAll(ctx context.Context) ([]models.AttestationKey, error) {
	var out []models.AttestationKey
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list attestation keys: %w", err)
	}
	return out, nil
}

// RetireActiveKeys flips every currently-active key to retired, so a newly
// created key becomes the sole active signer (REQ-006 rotation).
func (r *AttestationKeyRepo) RetireActiveKeys(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Model(&models.AttestationKey{}).
		Where("status = ?", models.AttestationKeyActive).
		Update("status", models.AttestationKeyRetired).Error; err != nil {
		return fmt.Errorf("retire active attestation keys: %w", err)
	}
	return nil
}
