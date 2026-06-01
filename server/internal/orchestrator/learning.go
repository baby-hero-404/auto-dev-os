package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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
}

func NewLearningEngine(memorySvc *service.MemoryService, suggestionSvc *service.LearningService, taskRepo *repository.TaskRepo) *LearningEngine {
	return &LearningEngine{
		memorySvc:     memorySvc,
		suggestionSvc: suggestionSvc,
		taskRepo:      taskRepo,
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

// DetectPatterns scans recent memories for recurring patterns and proposes skills.
func (le *LearningEngine) DetectPatterns(ctx context.Context, agentID string) {
	if le.memorySvc == nil || le.suggestionSvc == nil {
		return
	}

	// Look for repeated tool_sequence memories
	memories, err := le.memorySvc.ListByAgent(ctx, agentID, models.MemoryTierWorking, 50, 0)
	if err != nil {
		slog.Warn("learning: failed to list memories for pattern detection", "error", err)
		return
	}

	// Count category occurrences to find patterns
	categoryCounts := make(map[string]int)
	for _, mem := range memories {
		key := mem.Category + ":" + strings.Join(mem.Tags, ",")
		categoryCounts[key]++
	}

	for pattern, count := range categoryCounts {
		if count >= 3 { // Pattern threshold
			parts := strings.SplitN(pattern, ":", 2)
			category := parts[0]
			tags := ""
			if len(parts) > 1 {
				tags = parts[1]
			}

			input := models.CreateSuggestionInput{
				AgentID:        agentID,
				SuggestionType: models.SuggestionTypePattern,
				Title:          fmt.Sprintf("Recurring pattern detected: %s", category),
				Description:    fmt.Sprintf("Category '%s' with tags [%s] appeared %d times in recent executions. Consider extracting as a reusable skill.", category, tags, count),
				Content:        fmt.Sprintf("Pattern: %s (tags: %s), occurrences: %d", category, tags, count),
				Confidence:     clampConfidence(float64(count) * 0.15),
			}

			if _, err := le.suggestionSvc.CreateSuggestion(ctx, input); err != nil {
				slog.Warn("learning: failed to create pattern suggestion", "error", err)
			}
		}
	}
}

// SuggestRuleFromErrors analyzes repeated error patterns and proposes new rules.
func (le *LearningEngine) SuggestRuleFromErrors(ctx context.Context, agentID string) {
	if le.memorySvc == nil || le.suggestionSvc == nil {
		return
	}

	// Search for error memories
	results, err := le.memorySvc.Search(ctx, models.MemorySearchInput{
		Query:   "error failed retry",
		AgentID: agentID,
		Limit:   20,
	})
	if err != nil {
		slog.Warn("learning: failed to search error memories", "error", err)
		return
	}

	if len(results) < 3 {
		return // Not enough errors to form a rule suggestion
	}

	// Compile error themes
	var errorSummaries []string
	var projectID *string
	for _, r := range results {
		if r.Memory.Category == models.MemoryCategoryError {
			errorSummaries = append(errorSummaries, r.Memory.Summary)
			if projectID == nil {
				projectID = r.Memory.ProjectID
			}
		}
	}

	if len(errorSummaries) < 2 {
		return
	}

	input := models.CreateSuggestionInput{
		AgentID:        agentID,
		ProjectID:      projectID,
		SuggestionType: models.SuggestionTypeRule,
		Title:          "Recurring errors detected — rule suggestion",
		Description:    fmt.Sprintf("Found %d error memories. Consider adding a project rule to prevent these patterns.", len(errorSummaries)),
		Content:        "Suggested rule: " + strings.Join(errorSummaries[:min(3, len(errorSummaries))], "; "),
		Confidence:     clampConfidence(float64(len(errorSummaries)) * 0.1),
	}

	if _, err := le.suggestionSvc.CreateSuggestion(ctx, input); err != nil {
		slog.Warn("learning: failed to create rule suggestion", "error", err)
	}
}

// SuggestPromptPatch proposes system prompt modifications when tasks consistently fail.
func (le *LearningEngine) SuggestPromptPatch(ctx context.Context, task *models.Task, job *models.WorkflowJob) {
	if le.suggestionSvc == nil || job == nil || job.Attempts <= 2 {
		return
	}

	input := models.CreateSuggestionInput{
		AgentID:        safeAgentID(job.AgentID),
		ProjectID:      &task.ProjectID,
		TaskID:         &task.ID,
		SuggestionType: models.SuggestionTypePromptPatch,
		Title:          fmt.Sprintf("Prompt patch suggested for task '%s'", task.Title),
		Description:    fmt.Sprintf("Task failed after %d attempts. Last error: %s. Consider adjusting the agent's system prompt.", job.Attempts, job.LastError),
		Content:        fmt.Sprintf("Last error context: %s\nSuggested action: Add explicit instruction to handle '%s' scenarios in the system prompt.", job.LastError, extractErrorTheme(job.LastError)),
		Confidence:     clampConfidence(float64(job.Attempts) * 0.2),
	}

	if _, err := le.suggestionSvc.CreateSuggestion(ctx, input); err != nil {
		slog.Warn("learning: failed to create prompt patch suggestion", "error", err)
	}
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

func extractErrorTheme(lastError string) string {
	lower := strings.ToLower(lastError)
	themes := map[string]string{
		"timeout":     "timeout handling",
		"permission":  "permission/access control",
		"not found":   "resource validation",
		"syntax":      "code syntax",
		"connection":  "connection management",
		"nil pointer": "null safety",
	}
	for keyword, theme := range themes {
		if strings.Contains(lower, keyword) {
			return theme
		}
	}
	return "error handling"
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MarshalConfidenceToCheckpoint serializes confidence into a checkpoint-compatible map.
func MarshalConfidenceToCheckpoint(confidence float64) map[string]any {
	raw, _ := json.Marshal(map[string]any{"agent_confidence": confidence})
	return map[string]any{"confidence": confidence, "raw": string(raw)}
}
