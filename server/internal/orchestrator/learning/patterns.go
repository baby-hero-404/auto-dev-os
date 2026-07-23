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

// SuggestSkillFromTask proposes a reusable skill definition when a completed
// task's memory record shows a non-trivial multi-step tool sequence. Mirrors
// Hermes's `/learn` pattern: no separate distillation model call — this just
// formats a CreateSuggestionInput from data already collected by
// PostStepRecord/EvaluateOutcome, same as DetectPatterns does for pattern
// suggestions. The suggestion always lands as "proposed" for HITL review;
// this never registers a live, callable skill on its own.
func (le *LearningEngine) SuggestSkillFromTask(ctx context.Context, task *models.Task, job *models.WorkflowJob) {
	if le.memorySvc == nil || le.suggestionSvc == nil || task == nil {
		return
	}

	memories, err := le.memorySvc.ListByAgent(ctx, safeAgentID(job.AgentID), models.MemoryTierWorking, 200, 0)
	if err != nil {
		slog.Warn("learning: failed to list memories for skill suggestion", "error", err)
		return
	}

	stepSequence, toolCalls := taskToolCallSummary(task.ID, memories)

	if len(toolCalls) < 3 {
		return // not a non-trivial multi-tool-call task
	}

	taskID := task.ID
	input := models.CreateSuggestionInput{
		AgentID:        safeAgentID(job.AgentID),
		ProjectID:      &task.ProjectID,
		TaskID:         &taskID,
		SuggestionType: models.SuggestionTypeSkill,
		Title:          fmt.Sprintf("Reusable skill candidate from task '%s'", task.Title),
		Description:    fmt.Sprintf("Task made %d distinct tool calls across steps (%s). Consider extracting it as a reusable skill.", len(toolCalls), strings.Join(stepSequence, " -> ")),
		Content:        fmt.Sprintf("Proposed skill steps: %s\nTool calls: %s", strings.Join(stepSequence, " -> "), strings.Join(toolCalls, ", ")),
		Confidence:     clampConfidence(float64(len(toolCalls)) * 0.1),
	}

	if _, err := le.suggestionSvc.CreateSuggestion(ctx, input); err != nil {
		slog.Warn("learning: failed to create skill suggestion", "error", err)
	}
}

// ExtractLearnedSkills proposes up to 2 reusable LearnedSkill records (REQ-001)
// when a task merges, distinct from SuggestSkillFromTask above: that method
// only ever creates a generic HITL suggestion for a human to later turn into
// something; this creates directly loadable learned_skills rows (draft under
// supervised autonomy, active under autonomous), reusing the same
// tool-call/step-sequence summary data so DetectPatterns's own suggestion
// pipeline is unaffected (REQ-M01). Dedup: a candidate whose title shares any
// trigger-keyword token with an existing skill for the project is treated as
// reinforcing evidence (usage_count bump) rather than a new row.
func (le *LearningEngine) ExtractLearnedSkills(ctx context.Context, task *models.Task, autonomous bool) {
	if le.memorySvc == nil || le.learnedSkills == nil || task == nil {
		return
	}

	memories, err := le.memorySvc.ListByAgent(ctx, safeAgentID(task.AgentID), models.MemoryTierWorking, 200, 0)
	if err != nil {
		slog.Warn("learning: failed to list memories for skill extraction", "error", err)
		return
	}

	stepSequence, toolCalls := taskToolCallSummary(task.ID, memories)
	if len(toolCalls) < 3 {
		return // not a non-trivial multi-tool-call task
	}

	existing, err := le.learnedSkills.ListByProjectID(ctx, task.ProjectID)
	if err != nil {
		slog.Warn("learning: failed to list existing learned skills for dedup", "error", err)
		existing = nil
	}

	status := models.LearnedSkillStatusDraft
	if autonomous {
		status = models.LearnedSkillStatusActive
	}

	// Design caps extraction at max 2 candidates per task; a single tool-call
	// summary only ever yields one natural candidate title (the step
	// sequence), so this loop is a single iteration today but left as a
	// bounded loop to make the "max 2" ceiling explicit if a future pass
	// splits the summary into multiple candidates.
	candidates := []struct {
		title    string
		keywords []string
		content  string
	}{
		{
			title:    fmt.Sprintf("%s workflow", strings.Join(stepSequence, " -> ")),
			keywords: append(append([]string{}, stepSequence...), task.Labels...),
			content:  fmt.Sprintf("Step sequence: %s\nTool calls: %s", strings.Join(stepSequence, " -> "), strings.Join(toolCalls, ", ")),
		},
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}

	taskID := task.ID
	for _, cand := range candidates {
		if dup := findDuplicateSkill(existing, cand.title, cand.keywords); dup != nil {
			if err := le.learnedSkills.IncrementUsage(ctx, []string{dup.ID}, true); err != nil {
				slog.Warn("learning: failed to reinforce duplicate skill", "error", err)
			}
			continue
		}
		input := models.CreateLearnedSkillInput{
			ProjectID:       task.ProjectID,
			Title:           cand.title,
			TriggerKeywords: cand.keywords,
			Content:         cand.content,
			Status:          status,
			SourceTaskID:    &taskID,
		}
		if _, err := le.learnedSkills.Create(ctx, input); err != nil {
			slog.Warn("learning: failed to create learned skill", "error", err)
		}
	}
}

// findDuplicateSkill returns the first existing skill sharing at least one
// trigger-keyword token (case-insensitive) with the candidate, or nil.
func findDuplicateSkill(existing []models.LearnedSkill, title string, keywords []string) *models.LearnedSkill {
	candidateTokens := make(map[string]bool)
	for _, k := range keywords {
		candidateTokens[strings.ToLower(k)] = true
	}
	for _, tok := range strings.Fields(strings.ToLower(title)) {
		candidateTokens[tok] = true
	}

	for i := range existing {
		for _, k := range existing[i].TriggerKeywords {
			if candidateTokens[strings.ToLower(k)] {
				return &existing[i]
			}
		}
	}
	return nil
}

// RecordSkillOutcome updates usage/success counters (REQ-003) for the given
// learned-skill IDs when the task that loaded them reaches a terminal state.
// Best-effort: errors are logged, never propagated to the task lifecycle.
func (le *LearningEngine) RecordSkillOutcome(ctx context.Context, skillIDs []string, success bool) {
	if le.learnedSkills == nil || len(skillIDs) == 0 {
		return
	}
	if err := le.learnedSkills.IncrementUsage(ctx, skillIDs, success); err != nil {
		slog.Warn("learning: failed to record skill outcome", "error", err)
	}
}

// taskToolCallSummary extracts, from a task's step-observation memories, the
// ordered distinct workflow steps the task went through and the distinct
// tool-call proxies made along the way. Coding steps (CodeBackendStep et al)
// record a "files_changed" entry in PostStepRecord's output for every file an
// edit tool call (search_replace/create_file) touched; each distinct touched
// file stands in for a distinct tool call, since the memory system does not
// persist individual tool invocations directly (see memory_hooks.go
// PostStepRecord, which only tags step-level observations).
func taskToolCallSummary(taskID string, memories []models.EpisodicMemory) (stepSequence []string, toolCalls []string) {
	seenSteps := make(map[string]bool)
	seenFiles := make(map[string]bool)

	for _, mem := range memories {
		if mem.TaskID == nil || *mem.TaskID != taskID {
			continue
		}
		if mem.Category != models.MemoryCategoryObservation && mem.Category != models.MemoryCategorySuccess {
			continue
		}
		// PostStepRecord tags every step observation as ["workflow", stepID, status].
		if len(mem.Tags) < 2 {
			continue
		}
		stepID := mem.Tags[1]
		if !seenSteps[stepID] {
			seenSteps[stepID] = true
			stepSequence = append(stepSequence, stepID)
		}

		for _, f := range extractChangedFiles(mem.Content) {
			if !seenFiles[f] {
				seenFiles[f] = true
				toolCalls = append(toolCalls, f)
			}
		}
	}

	return stepSequence, toolCalls
}

// extractChangedFiles parses the "files_changed: [a.go b.go]" line that
// PostStepRecord's content-building loop produces from a coding step's
// output map (out["files_changed"] = changedFiles), returning the listed
// file paths. Returns nil if the memory content contains no such line.
func extractChangedFiles(content string) []string {
	const marker = "files_changed: ["
	_, rest, found := strings.Cut(content, marker)
	if !found {
		return nil
	}
	inner, _, found := strings.Cut(rest, "]")
	if !found {
		return nil
	}
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil
	}
	return strings.Fields(inner)
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
