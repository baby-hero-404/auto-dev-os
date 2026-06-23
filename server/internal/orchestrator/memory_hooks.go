package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/google/uuid"
)

// MemoryHooks provides lifecycle hooks for recording agent memories during workflow execution.
type MemoryHooks struct {
	memorySvc *service.MemoryService
}

func NewMemoryHooks(memorySvc *service.MemoryService) *MemoryHooks {
	return &MemoryHooks{memorySvc: memorySvc}
}

// SessionStart loads relevant memories for a task and returns them as context snippets.
// Called after agent assignment, before workflow execution begins.
func (h *MemoryHooks) SessionStart(ctx context.Context, agentID string, task *models.Task) ([]models.EpisodicMemory, error) {
	if h.memorySvc == nil {
		return nil, nil
	}

	query := task.Title + " " + task.Description
	results, err := h.memorySvc.Search(ctx, models.MemorySearchInput{
		Query:   query,
		AgentID: agentID,
		Limit:   5,
	})
	if err != nil {
		slog.Warn("memory hook: failed to retrieve memories at session start", "error", err)
		return nil, nil // non-fatal
	}

	memories := make([]models.EpisodicMemory, len(results))
	for i, r := range results {
		memories[i] = r.Memory
	}

	slog.Info("memory hook: loaded session context", "agent_id", agentID, "memories_loaded", len(memories))
	return memories, nil
}

// PostStepRecord records an observation after a workflow step completes.
// Called after each step in the workflow engine.
func (h *MemoryHooks) PostStepRecord(ctx context.Context, agentID string, task *models.Task, sessionID, stepID, status string, output map[string]any) {
	if h.memorySvc == nil {
		return
	}

	category := models.MemoryCategoryObservation
	if status == "failed" {
		category = models.MemoryCategoryError
	} else if stepID == "done" {
		category = models.MemoryCategorySuccess
	}

	content := fmt.Sprintf("Step '%s' completed with status '%s'.", stepID, status)
	// Include key output fields
	for k, v := range output {
		content += fmt.Sprintf("\n%s: %v", k, v)
	}

	taskID := task.ID
	input := models.CreateMemoryInput{
		AgentID:   agentID,
		ProjectID: &task.ProjectID,
		TaskID:    &taskID,
		SessionID: &sessionID,
		Tier:      models.MemoryTierWorking,
		Content:   content,
		Summary:   fmt.Sprintf("Step %s: %s", stepID, status),
		Category:  category,
		Tags:      []string{"workflow", stepID, status},
	}

	if _, err := h.memorySvc.RecordObservation(ctx, input); err != nil {
		slog.Warn("memory hook: failed to record step observation", "step", stepID, "error", err)
	}
}

// SessionEnd compiles a session summary, promotes important memories, and cleans up working tier.
// Called after the entire workflow run completes (success or failure).
func (h *MemoryHooks) SessionEnd(ctx context.Context, agentID string, task *models.Task, sessionID, finalStatus string) {
	if h.memorySvc == nil {
		return
	}

	// Record a final session summary as an episodic memory
	taskID := task.ID
	summary := fmt.Sprintf("Task '%s' (complexity: %s) completed with status: %s", task.Title, task.Complexity, finalStatus)
	category := models.MemoryCategorySuccess
	if finalStatus == "failed" {
		category = models.MemoryCategoryError
	}

	input := models.CreateMemoryInput{
		AgentID:   agentID,
		ProjectID: &task.ProjectID,
		TaskID:    &taskID,
		SessionID: &sessionID,
		Tier:      models.MemoryTierEpisodic, // Directly episodic — this is a session summary
		Content:   summary,
		Summary:   summary,
		Category:  category,
		Tags:      []string{"session_summary", finalStatus, task.Complexity},
	}

	if _, err := h.memorySvc.RecordObservation(ctx, input); err != nil {
		slog.Warn("memory hook: failed to record session summary", "error", err)
	}

	// Cleanup: promote important working memories, prune excess
	if err := h.memorySvc.CleanupSession(ctx, agentID, sessionID); err != nil {
		slog.Warn("memory hook: failed to cleanup session", "error", err)
	}

	slog.Info("memory hook: session ended", "agent_id", agentID, "task_id", task.ID, "status", finalStatus)
}

// NewSessionID generates a unique session identifier for a workflow run.
func NewSessionID() string {
	return uuid.New().String()
}
