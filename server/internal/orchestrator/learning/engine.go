package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// LearningEngine evaluates task outcomes and generates self-improvement suggestions.
// All suggestions require HITL approval before being applied (Option B).
type LearningEngine struct {
	memorySvc     *service.MemoryService
	suggestionSvc *service.LearningService
	taskRepo      *repository.TaskRepo
	learnedSkills *repository.LearnedSkillRepo
}

func NewLearningEngine(memorySvc *service.MemoryService, suggestionSvc *service.LearningService, taskRepo *repository.TaskRepo, learnedSkills *repository.LearnedSkillRepo) *LearningEngine {
	return &LearningEngine{
		memorySvc:     memorySvc,
		suggestionSvc: suggestionSvc,
		taskRepo:      taskRepo,
		learnedSkills: learnedSkills,
	}
}

// EvaluateOutcome classifies task execution results and records them as memories.
// Called after every workflow run completes.
func (le *LearningEngine) EvaluateOutcome(ctx context.Context, task *models.Task, job *models.WorkflowJob) {
	if le.memorySvc == nil || job == nil {
		return
	}

	outcome := classifyOutcome(job)

	// Record the evaluation as a decision memory
	taskID := task.ID
	input := models.CreateMemoryInput{
		AgentID:   safeAgentID(job.AgentID),
		ProjectID: &task.ProjectID,
		TaskID:    &taskID,
		Tier:      models.MemoryTierEpisodic,
		Content:   fmt.Sprintf("Task '%s' outcome: %s. Status: %s, Attempts: %d, Last error: %s", task.Title, outcome, job.Status, job.Attempts, job.LastError),
		Summary:   fmt.Sprintf("Outcome: %s for task '%s'", outcome, task.Title),
		Category:  outcomeCategory(outcome),
		Tags:      []string{"evaluation", outcome, task.Complexity},
	}

	if _, err := le.memorySvc.RecordObservation(ctx, input); err != nil {
		slog.Warn("learning: failed to record evaluation", "error", err)
	}

	slog.Info("learning: evaluated outcome", "task_id", task.ID, "outcome", outcome, "attempts", job.Attempts)
}

// ComputeConfidence calculates a confidence score for an agent on a given task type.
// Returns a value between 0.0 and 1.0 based on historical success rate.
func (le *LearningEngine) ComputeConfidence(ctx context.Context, agentID, complexity string) float64 {
	if le.memorySvc == nil {
		return 0.5 // Default confidence
	}

	// Search for past evaluation memories for this agent
	results, err := le.memorySvc.Search(ctx, models.MemorySearchInput{
		Query:   "evaluation outcome " + complexity,
		AgentID: agentID,
		Limit:   20,
	})
	if err != nil || len(results) == 0 {
		return 0.5
	}

	successCount := 0
	totalCount := 0
	for _, r := range results {
		if r.Memory.Category == models.MemoryCategorySuccess {
			successCount++
		}
		totalCount++
	}

	if totalCount == 0 {
		return 0.5
	}

	// Weight by complexity
	baseRate := float64(successCount) / float64(totalCount)
	complexityWeight := 1.0
	switch complexity {
	case models.TaskComplexityMedium:
		complexityWeight = 0.85
	case models.TaskComplexityHard:
		complexityWeight = 0.7
	}

	return clampConfidence(baseRate * complexityWeight)
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal Helpers
// ──────────────────────────────────────────────────────────────────────────────

func classifyOutcome(job *models.WorkflowJob) string {
	switch {
	case job.Status == models.WorkflowJobStatusDone:
		if job.Attempts > 1 {
			return "success_with_retries"
		}
		return "success"
	case job.Status == models.WorkflowJobStatusFailed:
		return "failure"
	case job.Status == models.WorkflowJobStatusPaused:
		return "paused"
	default:
		return "unknown"
	}
}

func outcomeCategory(outcome string) string {
	switch outcome {
	case "success", "success_with_retries":
		return models.MemoryCategorySuccess
	case "failure":
		return models.MemoryCategoryError
	default:
		return models.MemoryCategoryDecision
	}
}

func safeAgentID(agentID *string) string {
	if agentID == nil {
		return ""
	}
	return *agentID
}

func clampConfidence(v float64) float64 {
	if v < 0.0 {
		return 0.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}

// MarshalConfidenceToCheckpoint serializes confidence into a checkpoint-compatible map.
func MarshalConfidenceToCheckpoint(confidence float64) map[string]any {
	raw, _ := json.Marshal(map[string]any{"agent_confidence": confidence})
	return map[string]any{"confidence": confidence, "raw": string(raw)}
}
