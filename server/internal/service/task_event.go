package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TaskEventService records and streams a task's append-only agent-activity
// event log (docs/openspecs/status-driven-agent-workspace).
type TaskEventService struct {
	repo *repository.TaskEventRepo
}

func NewTaskEventService(repo *repository.TaskEventRepo) *TaskEventService {
	return &TaskEventService{repo: repo}
}

// Emit persists an event and broadcasts it to any live SSE subscribers.
// Payloads over models.MaxPayloadBytes must be externalized by the caller
// (via workflow_artifacts + ArtifactID) before calling Emit — this method
// does not perform externalization itself.
func (s *TaskEventService) Emit(ctx context.Context, taskID, eventType string, schemaVersion int, payload json.RawMessage, artifactID *string) (*models.TaskEvent, error) {
	if len(payload) > models.MaxPayloadBytes {
		return nil, fmt.Errorf("emit task event: payload of %d bytes exceeds MaxPayloadBytes (%d); externalize via workflow_artifacts first", len(payload), models.MaxPayloadBytes)
	}

	event := &models.TaskEvent{
		TaskID:        taskID,
		Type:          eventType,
		SchemaVersion: schemaVersion,
		Payload:       payload,
		ArtifactID:    artifactID,
		SizeBytes:     len(payload),
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *TaskEventService) ListByTaskID(ctx context.Context, taskID string) ([]models.TaskEvent, error) {
	return s.repo.ListByTaskID(ctx, taskID)
}

func (s *TaskEventService) ListByTaskIDAfter(ctx context.Context, taskID string, after int64) ([]models.TaskEvent, error) {
	return s.repo.ListByTaskIDAfter(ctx, taskID, after)
}

func (s *TaskEventService) ListByTaskIDPaginated(ctx context.Context, taskID string, before int64, limit int) ([]models.TaskEvent, error) {
	return s.repo.ListByTaskIDPaginated(ctx, taskID, before, limit)
}

func (s *TaskEventService) CountByTaskID(ctx context.Context, taskID string) (int64, error) {
	return s.repo.CountByTaskID(ctx, taskID)
}

func (s *TaskEventService) Subscribe(taskID string) chan models.TaskEvent {
	return s.repo.Subscribe(taskID)
}

func (s *TaskEventService) Unsubscribe(taskID string, ch chan models.TaskEvent) {
	s.repo.Unsubscribe(taskID, ch)
}
