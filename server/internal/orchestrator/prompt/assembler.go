package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type PromptAssembler struct {
	rules     *repository.RuleRepo
	skills    SkillLister
	baseTools []llm.ToolDefinition
	root      string
	dataRoot  string
	ctxEngine provider.ContextEngine
}

type SkillLister interface {
	List(context.Context) ([]models.Skill, error)
}

func NewPromptAssembler(baseTools []llm.ToolDefinition, ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{baseTools: baseTools, root: ".", ctxEngine: ctxEngine}
}

func NewPromptAssemblerWithRules(rules *repository.RuleRepo, baseTools []llm.ToolDefinition, root string, ctxEngine provider.ContextEngine) *PromptAssembler {
	if root == "" {
		root = "."
	}
	return &PromptAssembler{rules: rules, baseTools: baseTools, root: root, ctxEngine: ctxEngine}
}

func (a *PromptAssembler) WithSkillLister(skills SkillLister) *PromptAssembler {
	a.skills = skills
	return a
}

func (a *PromptAssembler) WithDataRoot(dataRoot string) *PromptAssembler {
	a.dataRoot = dataRoot
	return a
}

func (a *PromptAssembler) Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error) {
	return a.AssembleForAgent(ctx, task, nil, nil)
}

type contextKey string

const MemoriesCtxKey contextKey = "retrieved_memories"
const StepIDCtxKey contextKey = "prompt_step_id"

func stepIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(StepIDCtxKey).(string); ok {
		return v
	}
	return ""
}

// shouldInjectFullSpec returns true for steps that need the full OpenSpec
// (analyze, plan, review). Coding and fix steps already get the relevant
// subtask text injected by the step runner itself.
func shouldInjectFullSpec(stepID string) bool {
	return stepID == "" ||
		stepID == workflow.StepAnalyze ||
		stepID == workflow.StepPlan ||
		stepID == workflow.StepReview ||
		stepID == workflow.StepContextLoad
}

// isCodingStep returns true for steps that produce code patches.
func isCodingStep(stepID string) bool {
	return strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend) ||
		stepID == workflow.StepFix
}

func (a *PromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error) {
	stepID := stepIDFromCtx(ctx)

	var contextBlock string
	var activeFiles []string
	if a != nil && a.ctxEngine != nil && shouldAttachCodeContext(agent) {
		maxSnippets := 8
		if isCodingStep(stepID) {
			maxSnippets = 4
		}
		snippets, err := a.ctxEngine.RetrieveContext(ctx, task.Title+"\n"+task.Description, maxSnippets)
		if err != nil {
			return nil, nil, err
		}
		snippets = deduplicateSnippets(snippets)
		contextBlock = formatContextSnippets(snippets)
		
		for _, s := range snippets {
			activeFiles = append(activeFiles, s.Path)
		}
	}

	// Inject Project Knowledge Base Docs (Planned 5.5)
	if a != nil && a.dataRoot != "" && shouldAttachCodeContext(agent) {
		kbContent := a.loadProjectKnowledgeBaseDocs(task.ProjectID, task.Title+"\n"+task.Description)
		if kbContent != "" {
			if contextBlock != "" {
				contextBlock = kbContent + "\n\n" + contextBlock
			} else {
				contextBlock = kbContent
			}
		}
	}

	system, _, err := a.systemPrompt(ctx, task, agent)
	if err != nil {
		return nil, nil, err
	}

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	// Extract active targets safely from structured ExecutionPlan
	if len(analysis.ExecutionPlan) > 0 {
		for _, step := range analysis.ExecutionPlan {
			words := strings.Fields(step)
			for _, w := range words {
				cleanWord := strings.Trim(w, "`,.\":")
				// Simple heuristic to catch file paths
				if strings.Contains(cleanWord, ".") && !strings.HasSuffix(cleanWord, ".") {
					activeFiles = append(activeFiles, cleanWord)
				}
			}
		}
	}
	
	// Deduplicate activeFiles
	seenFiles := make(map[string]bool)
	var uniqueActiveFiles []string
	for _, f := range activeFiles {
		if !seenFiles[f] {
			seenFiles[f] = true
			uniqueActiveFiles = append(uniqueActiveFiles, f)
		}
	}
	activeFiles = uniqueActiveFiles

	user := "Task: " + task.Title + "\n\n" + task.Description
	
	// Only inject full OpenSpec for analysis/planning/review steps.
	// Coding steps already get the relevant subtask text from the step runner.
	if shouldInjectFullSpec(stepID) {
		if analysis.ProposalMD != "" || analysis.SpecsMD != "" || len(analysis.ExecutionPlan) > 0 {
			user += "\n\n=== Task Specification (OpenSpec) ===\n"
			if analysis.ProposalMD != "" {
				user += analysis.ProposalMD + "\n\n"
			}
			if analysis.SpecsMD != "" {
				user += analysis.SpecsMD + "\n\n"
			}
			if analysis.DesignMD != "" {
				user += analysis.DesignMD + "\n\n"
			}
			if analysis.TasksMD != "" {
				user += analysis.TasksMD + "\n\n"
			}
			if len(analysis.ExecutionPlan) > 0 {
				user += "## Initial Execution Plan:\n"
				for _, p := range analysis.ExecutionPlan {
					user += "- " + p + "\n"
				}
				user += "\n"
			}
		}
	} else if isCodingStep(stepID) {
		idx, ok := extractSubtaskIndex(stepID)
		if ok && idx >= 0 {
			specSection := extractSpecsSectionForSubtask(analysis.SpecsMD, analysis.TasksMD, idx, stepID)
			if specSection != "" {
				user += "\n\n=== Relevant Requirements (OpenSpec) ===\n" + specSection + "\n\n"
			}
			progress := summarizeTasksProgress(analysis.TasksMD, idx, stepID)
			if progress != "" {
				user += progress + "\n\n"
			}
		}
	}

	if contextBlock != "" {
		user += "\n\nSemantic Code Retrieval Context:\n" + contextBlock
	}
	if memories, ok := ctx.Value(MemoriesCtxKey).([]models.EpisodicMemory); ok && len(memories) > 0 {
		user += "\n\nRetrieved Memories:\n" + formatMemories(memories)
	}

	// TOKEN BUDGET ARBITRATION
	if a != nil && a.ctxEngine != nil {
		if agent != nil && (agent.Role == models.AgentRoleBackend || agent.Role == models.AgentRoleFrontend || agent.Role == models.AgentRoleReviewer) {
			// Heuristic: 1 token ~= 4 chars
			usedTokens := len(user) / 4
			totalBudget := 8192 // Target system budget
			maxMapTokens := totalBudget - usedTokens
			
			if maxMapTokens > 2048 {
				maxMapTokens = 2048
			} else if maxMapTokens < 256 {
				maxMapTokens = 256 // Fallback minimum
			}
			
			repoMap, err := a.ctxEngine.GetRepoMap(ctx, activeFiles, maxMapTokens)
			if err == nil && repoMap != "" {
				user = "=== Repository Structure ===\n" + repoMap + "\n\n" + user
			}
		}
	}
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	messages = append(messages, TruncateHistory(history, 12000)...)
	tools, err := a.toolDefinitionsForAgent(ctx, agent, task.ProjectID, analysis.RequiredSkills)
	if err != nil {
		return nil, nil, err
	}
	return messages, tools, nil
}

func (a *PromptAssembler) systemPrompt(ctx context.Context, task models.Task, agent *models.Agent) (string, []models.Rule, error) {
	stepID := stepIDFromCtx(ctx)
	root := "."
	if a != nil && a.root != "" {
		root = a.root
	}
	parts := []string{}
	if content, err := readOptional(filepath.Join(root, "core", "system_prompt.md")); err == nil && strings.TrimSpace(content) != "" {
		parts = append(parts, "# Base System Prompt\n"+content)
	}

	globalRules, projectRules, err := a.loadRules(ctx, task.ProjectID)
	if err != nil {
		return "", nil, err
	}
	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	localRules := append([]models.Rule{}, projectRules...)
	for i, tr := range analysis.TaskRules {
		localRules = append(localRules, models.Rule{
			ID:          fmt.Sprintf("task-rule-%d", i),
			Scope:       "task",
			Content:     tr,
			Enforcement: models.RuleEnforcementStrict,
		})
	}
	if err := DetectRuleConflicts(globalRules, localRules); err != nil {
		return "", nil, err
	}

	// 1. Global Rules (filtered for coding steps to remove impossible instructions)
	if len(globalRules) > 0 {
		filtered := filterRulesForStep(globalRules, stepID)
		if len(filtered) > 0 {
			parts = append(parts, "# Global Rules [IMMUTABLE - DO NOT OVERRIDE]\n"+formatRules(filtered))
		}
	}

	// 2. Agent Role Constraints
	if agent != nil {
		if content, err := readOptional(filepath.Join(root, "roles", roleFile(agent.Role))); err == nil && strings.TrimSpace(content) != "" {
			parts = append(parts, "# Agent Role Constraints\n"+content)
		}
	}

	// 3. Project Rules
	if len(projectRules) > 0 {
		parts = append(parts, "# Project Rules\n"+formatRules(projectRules))
	}

	// 4. Task Rules
	if len(analysis.TaskRules) > 0 {
		var b strings.Builder
		b.WriteString("# Task-specific Rules:\n")
		for _, tr := range analysis.TaskRules {
			b.WriteString(fmt.Sprintf("- [task/strict] %s\n", strings.TrimSpace(tr)))
		}
		parts = append(parts, b.String())
	}

	// Output Rules
	if content, err := readOptional(filepath.Join(root, "core", "output_rules.md")); err == nil && strings.TrimSpace(content) != "" {
		parts = append(parts, content)
	}

	corePrompt := strings.TrimSpace(strings.Join(parts, "\n\n"))
	
	// Dynamic Metadata Injection (appendSystemPrompt)
	metadata := map[string]any{
		"project_id": task.ProjectID,
		"task_id":    task.ID,
	}
	if agent != nil {
		metadata["assigned_role"] = agent.Role
	}
	if len(analysis.TaskRules) > 0 {
		metadata["task_rules"] = analysis.TaskRules
	}
	
	finalSystemPrompt := appendSystemPrompt(corePrompt, metadata)
	return finalSystemPrompt, projectRules, nil
}
