package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// AuditService handles recording and querying of immutable audit logs.
type AuditService struct {
	repo *repository.AuditRepo
}

func NewAuditService(repo *repository.AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

// Record creates an audit log entry. This is fire-and-forget safe; errors are logged but not propagated.
func (s *AuditService) Record(ctx context.Context, log models.AuditLog) {
	if err := s.repo.Record(ctx, log); err != nil {
		slog.Error("failed to record audit log", "action", log.Action, "error", err)
	}
}

// RecordAction is a convenience method for recording a simple audit action.
func (s *AuditService) RecordAction(ctx context.Context, action, entityType, entityID string, opts ...AuditOption) {
	entry := models.AuditLog{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    json.RawMessage("{}"),
	}
	for _, opt := range opts {
		opt(&entry)
	}
	s.Record(ctx, entry)
}

// List returns audit logs matching the given filters.
func (s *AuditService) List(ctx context.Context, filter models.AuditLogFilter) ([]models.AuditLog, error) {
	return s.repo.List(ctx, filter)
}

// CountByAction returns a count of audit logs grouped by action.
func (s *AuditService) CountByAction(ctx context.Context, orgID string) (map[string]int64, error) {
	return s.repo.CountByAction(ctx, orgID)
}

// AuditOption configures an audit log entry.
type AuditOption func(*models.AuditLog)

func WithOrgID(id string) AuditOption {
	return func(l *models.AuditLog) { l.OrgID = &id }
}

func WithUserID(id string) AuditOption {
	return func(l *models.AuditLog) { l.UserID = &id }
}

func WithAgentID(id string) AuditOption {
	return func(l *models.AuditLog) { l.AgentID = &id }
}

func WithTaskID(id string) AuditOption {
	return func(l *models.AuditLog) { l.TaskID = &id }
}

func WithIPAddress(ip string) AuditOption {
	return func(l *models.AuditLog) { l.IPAddress = ip }
}

func WithDetails(details any) AuditOption {
	return func(l *models.AuditLog) {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("marshal audit details", "error", err)
			return
		}
		l.Details = b
	}
}
