package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// LearningSuggestionRepo handles CRUD for learning suggestions (HITL queue).
type LearningSuggestionRepo struct{ db *gorm.DB }

func NewLearningSuggestionRepo(db *gorm.DB) *LearningSuggestionRepo {
	return &LearningSuggestionRepo{db: db}
}

// Create inserts a new learning suggestion.
func (r *LearningSuggestionRepo) Create(ctx context.Context, s *models.LearningSuggestion) error {
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("create learning suggestion: %w", err)
	}
	return nil
}

// GetByID fetches a single suggestion by ID.
func (r *LearningSuggestionRepo) GetByID(ctx context.Context, id string) (*models.LearningSuggestion, error) {
	var s models.LearningSuggestion
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get suggestion: %w", mapError(err))
	}
	return &s, nil
}

// List returns suggestions filtered by agent and/or status with pagination.
func (r *LearningSuggestionRepo) List(ctx context.Context, agentID, status string, limit int) ([]models.LearningSuggestion, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var suggestions []models.LearningSuggestion
	if err := query.Limit(limit).Find(&suggestions).Error; err != nil {
		return nil, fmt.Errorf("list suggestions: %w", err)
	}
	return suggestions, nil
}

// Update applies partial updates to a suggestion (approve/reject with reviewer info).
func (r *LearningSuggestionRepo) Update(ctx context.Context, id string, input models.UpdateSuggestionInput) (*models.LearningSuggestion, error) {
	s, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{"updated_at": time.Now()}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if input.ReviewedBy != nil {
		updates["reviewed_by"] = *input.ReviewedBy
		now := time.Now()
		updates["reviewed_at"] = now
	}
	if input.Feedback != nil {
		// Merge feedback into existing metadata
		meta := make(map[string]any)
		if len(s.Metadata) > 0 {
			_ = json.Unmarshal(s.Metadata, &meta)
		}
		meta["review_feedback"] = *input.Feedback
		raw, _ := json.Marshal(meta)
		updates["metadata"] = raw
	}

	if err := r.db.WithContext(ctx).Model(&models.LearningSuggestion{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update suggestion: %w", err)
	}

	return r.GetByID(ctx, id)
}
