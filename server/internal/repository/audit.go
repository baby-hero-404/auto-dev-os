package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// AuditRepo handles CRUD for immutable audit log entries.
type AuditRepo struct{ db *gorm.DB }

func NewAuditRepo(db *gorm.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// Record creates an immutable audit log entry.
func (r *AuditRepo) Record(ctx context.Context, log models.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(&log).Error; err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

// List returns audit logs matching the given filters with pagination.
func (r *AuditRepo) List(ctx context.Context, f models.AuditLogFilter) ([]models.AuditLog, error) {
	query := r.db.WithContext(ctx).Order("created_at DESC")

	if f.OrgID != "" {
		query = query.Where("org_id = ?", f.OrgID)
	}
	if f.UserID != "" {
		query = query.Where("user_id = ?", f.UserID)
	}
	if f.AgentID != "" {
		query = query.Where("agent_id = ?", f.AgentID)
	}
	if f.TaskID != "" {
		query = query.Where("task_id = ?", f.TaskID)
	}
	if f.Action != "" {
		query = query.Where("action = ?", f.Action)
	}
	if f.EntityType != "" {
		query = query.Where("entity_type = ?", f.EntityType)
	}
	if !f.Since.IsZero() {
		query = query.Where("created_at >= ?", f.Since)
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query = query.Limit(limit)

	var logs []models.AuditLog
	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	return logs, nil
}

// CountByAction returns a count of audit logs grouped by action.
func (r *AuditRepo) CountByAction(ctx context.Context, orgID string) (map[string]int64, error) {
	type result struct {
		Action string
		Count  int64
	}
	var results []result

	query := r.db.WithContext(ctx).Table("audit_logs").
		Select("action, COUNT(*) AS count").
		Group("action")
	if orgID != "" {
		query = query.Where("org_id = ?", orgID)
	}
	if err := query.Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("count audit by action: %w", err)
	}

	counts := make(map[string]int64, len(results))
	for _, r := range results {
		counts[r.Action] = r.Count
	}
	return counts, nil
}
