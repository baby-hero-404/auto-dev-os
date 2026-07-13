package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type Store struct {
	Workflows CheckpointRepo
	Artifacts ArtifactRepo
	Log       func(ctx context.Context, taskID string, jobID *string, level string, message string)
}

func NewStore(
	workflows CheckpointRepo,
	artifacts ArtifactRepo,
	log func(ctx context.Context, taskID string, jobID *string, level string, message string),
) *Store {
	return &Store{
		Workflows: workflows,
		Artifacts: artifacts,
		Log:       log,
	}
}

func (s *Store) GetSuccessful(ctx context.Context, taskID string, step string) (map[string]any, bool) {
	if s.Workflows == nil {
		return nil, false
	}
	checkpoints, err := s.Workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, false
	}
	var latestSuccess *models.WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step == step {
			var state map[string]any
			if err := json.Unmarshal(cp.State, &state); err == nil {
				if state["status"] == "success" {
					latestSuccess = &cp
					break
				}
			}
		}
	}
	if latestSuccess != nil {
		var state map[string]any
		_ = json.Unmarshal(latestSuccess.State, &state)
		if out, ok := state["output"].(map[string]any); ok {
			return out, true
		}
		return map[string]any{}, true
	}
	return nil, false
}

func (s *Store) CountSuccessful(ctx context.Context, taskID string, step string) int {
	if s.Workflows == nil {
		return 0
	}
	checkpoints, err := s.Workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return 0
	}
	count := 0
	for _, cp := range checkpoints {
		if cp.Step != step {
			continue
		}
		var state map[string]any
		if err := json.Unmarshal(cp.State, &state); err != nil {
			continue
		}
		if status, _ := state["status"].(string); status == workflow.StepStatusSuccess {
			count++
		}
	}
	return count
}

func (s *Store) GetSavedPatch(ctx context.Context, taskID string, step string) (string, error) {
	if s.Artifacts == nil {
		return "", fmt.Errorf("artifacts repository is not configured")
	}
	arts, err := s.Artifacts.ListByTaskID(ctx, taskID)
	if err != nil {
		return "", err
	}
	var latestPatch *models.WorkflowArtifact
	for i := len(arts) - 1; i >= 0; i-- {
		art := arts[i]
		if (art.Step == step || strings.HasPrefix(art.Step, step+"_cycle_")) && art.Type == "patch" {
			latestPatch = &art
			break
		}
	}
	if latestPatch == nil {
		return "", fmt.Errorf("no patch artifact found for step %s", step)
	}
	var patch string
	if err := json.Unmarshal(latestPatch.Payload, &patch); err == nil {
		return patch, nil
	}
	return string(latestPatch.Payload), nil
}

func (s *Store) SaveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	if s.Artifacts == nil {
		return nil
	}

	artifacts, err := s.Artifacts.ListByTaskID(ctx, taskID)
	if err == nil {
		count := 0
		for _, a := range artifacts {
			if (a.Step == step || strings.HasPrefix(a.Step, step+"_cycle_")) && a.Type == artType {
				count++
			}
		}
		if count > 0 {
			step = fmt.Sprintf("%s_cycle_%d", step, count+1)
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	artifact := &models.WorkflowArtifact{
		JobID:   jobID,
		TaskID:  taskID,
		Step:    step,
		Type:    artType,
		Payload: raw,
	}
	return s.Artifacts.Create(ctx, artifact)
}

func (s *Store) GetLatestExecutionSnapshot(ctx context.Context, taskID string, step string) (*models.ExecutionSnapshot, bool) {
	if s.Artifacts == nil {
		return nil, false
	}
	arts, err := s.Artifacts.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, false
	}
	var latest *models.WorkflowArtifact
	for i := len(arts) - 1; i >= 0; i-- {
		art := arts[i]
		if (art.Step == step || strings.HasPrefix(art.Step, step+"_cycle_")) && art.Type == "execution_snapshot" {
			latest = &art
			break
		}
	}
	if latest == nil {
		return nil, false
	}
	var snap models.ExecutionSnapshot
	if err := json.Unmarshal(latest.Payload, &snap); err == nil {
		return &snap, true
	}
	return nil, false
}
