package learning

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
