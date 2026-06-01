package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// KnowledgeEdgeRepo handles CRUD for directed knowledge graph edges between memories.
type KnowledgeEdgeRepo struct{ db *gorm.DB }

func NewKnowledgeEdgeRepo(db *gorm.DB) *KnowledgeEdgeRepo {
	return &KnowledgeEdgeRepo{db: db}
}

// Create inserts a new knowledge edge. Silently skips if duplicate (source, target, relation).
func (r *KnowledgeEdgeRepo) Create(ctx context.Context, edge *models.KnowledgeEdge) error {
	result := r.db.WithContext(ctx).
		Where("source_id = ? AND target_id = ? AND relation = ?", edge.SourceID, edge.TargetID, edge.Relation).
		FirstOrCreate(edge)
	if result.Error != nil {
		return fmt.Errorf("create knowledge edge: %w", result.Error)
	}
	return nil
}

// ListBySource returns all outbound edges from a given memory.
func (r *KnowledgeEdgeRepo) ListBySource(ctx context.Context, sourceID string) ([]models.KnowledgeEdge, error) {
	var edges []models.KnowledgeEdge
	if err := r.db.WithContext(ctx).Where("source_id = ?", sourceID).Order("weight DESC").Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("list edges by source: %w", err)
	}
	return edges, nil
}

// ListByTarget returns all inbound edges to a given memory.
func (r *KnowledgeEdgeRepo) ListByTarget(ctx context.Context, targetID string) ([]models.KnowledgeEdge, error) {
	var edges []models.KnowledgeEdge
	if err := r.db.WithContext(ctx).Where("target_id = ?", targetID).Order("weight DESC").Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("list edges by target: %w", err)
	}
	return edges, nil
}

// Delete removes a knowledge edge by ID.
func (r *KnowledgeEdgeRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.KnowledgeEdge{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete knowledge edge: %w", err)
	}
	return nil
}
