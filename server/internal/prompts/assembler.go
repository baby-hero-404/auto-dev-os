package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type PromptAssembler struct {
	rules       *repository.RuleRepo
	skills      SkillLister
	baseTools   []llm.ToolDefinition
	promptPaths paths.PromptPaths
	fs          paths.FileSystem
	dataRoot    string
	ctxEngine   provider.ContextEngine
}

type SkillLister interface {
	List(context.Context) ([]models.Skill, error)
}

func NewPromptAssembler(baseTools []llm.ToolDefinition, ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{
		baseTools:   baseTools,
		promptPaths: paths.NewOSPromptPaths("."),
		fs:          paths.NewOSFileSystem(),
		ctxEngine:   ctxEngine,
	}
}

func NewPromptAssemblerWithRules(rules *repository.RuleRepo, baseTools []llm.ToolDefinition, promptPaths paths.PromptPaths, fs paths.FileSystem, ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{
		rules:       rules,
		baseTools:   baseTools,
		promptPaths: promptPaths,
		fs:          fs,
		ctxEngine:   ctxEngine,
	}
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

	// Extract active targets safely from structured ExecutionPhases
	if len(analysis.ExecutionPhases) > 0 {
		for _, phase := range analysis.ExecutionPhases {
			for _, step := range phase.Tasks {
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

	user := "Task: " + task.Title + "\n\n"

	useOriginalDescription := true
	if (shouldInjectFullSpec(stepID) || isCodingStep(stepID) || stepID == workflow.StepReview || stepID == workflow.StepTest) && (analysis.SpecsMD != "" || analysis.ProposalMD != "") {
		useOriginalDescription = false
	}

	if useOriginalDescription {
		user += task.Description
	} else {
		user += "> [!IMPORTANT]\n> Original Task Description is omitted. Your evaluation MUST be based strictly on the execution contract and specific context provided in this prompt. Do NOT rely on prior assumptions.\n"
	}

	if len(task.Clarifications) > 0 {
		var rounds []models.ClarificationRound
		if err := json.Unmarshal(task.Clarifications, &rounds); err == nil && len(rounds) > 0 {
			user += "\n\n=== Answers to Clarification Questions ===\n"
			for _, r := range rounds {
				user += fmt.Sprintf("#### Round %d:\n%s\n\n", r.Round, r.Response)
			}
		}
	}

	if shouldInjectFullSpec(stepID) || isCodingStep(stepID) {
		if analysis.ProposalMD != "" || analysis.SpecsMD != "" || len(analysis.ExecutionPhases) > 0 {
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
			if len(analysis.Tasks) > 0 {
				user += formatTasksMD(analysis.Tasks) + "\n\n"
			} else if analysis.TasksMD != "" {
				user += analysis.TasksMD + "\n\n"
			}
			
			// Inject Execution Manifest (JSON)
			var manifestJSON []byte
			if isCodingStep(stepID) {
				manifest := map[string]any{
					"affected_files": analysis.AffectedFiles,
				}
				if len(analysis.Tasks) > 0 {
					manifest["tasks"] = analysis.Tasks
				}
				manifestJSON, _ = json.MarshalIndent(manifest, "", "  ")

				// Calculate before/after tokens for metric logging
				fullManifest := map[string]any{
					"affected_files": analysis.AffectedFiles,
					"risks":          analysis.Risks,
				}
				if len(analysis.ExecutionPhases) > 0 {
					fullManifest["execution_phases"] = analysis.ExecutionPhases
				}
				if len(analysis.Tasks) > 0 {
					fullManifest["tasks"] = analysis.Tasks
				}
				if len(analysis.RiskDomains) > 0 {
					fullManifest["risk_domains"] = analysis.RiskDomains
				}
				if len(analysis.AcceptanceCriteria) > 0 {
					fullManifest["acceptance_criteria"] = analysis.AcceptanceCriteria
				}
				if len(analysis.ExecutionBoundaries) > 0 {
					fullManifest["execution_boundaries"] = analysis.ExecutionBoundaries
				}
				fullJSON, _ := json.MarshalIndent(fullManifest, "", "  ")

				userBefore := user + "## Execution Manifest (JSON):\n```json\n" + string(fullJSON) + "\n```\n\n"
				userAfter := user + "## Execution Manifest (JSON):\n```json\n" + string(manifestJSON) + "\n```\n\n"
				tokensBefore := len(userBefore) / 4
				tokensAfter := len(userAfter) / 4
				log.Printf("[Metric] Prompt pruning for step %s: prompt_tokens_before=%d, prompt_tokens_after=%d, reduced=%d (%.2f%%)",
					stepID, tokensBefore, tokensAfter, tokensBefore-tokensAfter, float64(tokensBefore-tokensAfter)/float64(tokensBefore)*100.0)

				// Strip existing manifest from TasksMD
				re := regexp.MustCompile(`(?s)## Execution Manifest \(JSON\):\n*` + "```" + `json\n.*?` + "```" + `\n*`)
				user = re.ReplaceAllString(user, "")
			} else {
				manifest := map[string]any{
					"affected_files": analysis.AffectedFiles,
					"risks":          analysis.Risks,
				}
				if len(analysis.ExecutionPhases) > 0 {
					manifest["execution_phases"] = analysis.ExecutionPhases
				}
				if len(analysis.Tasks) > 0 {
					manifest["tasks"] = analysis.Tasks
				}
				if len(analysis.RiskDomains) > 0 {
					manifest["risk_domains"] = analysis.RiskDomains
				}
				if len(analysis.AcceptanceCriteria) > 0 {
					manifest["acceptance_criteria"] = analysis.AcceptanceCriteria
				}
				if len(analysis.ExecutionBoundaries) > 0 {
					manifest["execution_boundaries"] = analysis.ExecutionBoundaries
				}
				manifestJSON, _ = json.MarshalIndent(manifest, "", "  ")
			}
			if len(manifestJSON) > 0 {
				user += "## Execution Manifest (JSON):\n```json\n" + string(manifestJSON) + "\n```\n\n"
			}
		}
	}
	
	if isCodingStep(stepID) {
		idx, ok := extractSubtaskIndex(stepID)
		if ok && idx >= 0 {
			specSection := extractSpecsSectionForSubtask(analysis.SpecsMD, formatTasksMD(analysis.Tasks), idx, stepID)
			if specSection != "" {
				user += "\n\n=== Relevant Requirements (OpenSpec) ===\n" + specSection + "\n\n"
			}
			progress := summarizeTasksProgress(formatTasksMD(analysis.Tasks), idx, stepID)
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
	tools, err := a.toolDefinitionsForAgent(ctx, agent, task.ProjectID, analysis.RequiredSkills, analysis.RequiredSkillsMap)
	if err != nil {
		return nil, nil, err
	}
	return messages, tools, nil
}

func (a *PromptAssembler) systemPrompt(ctx context.Context, task models.Task, agent *models.Agent) (string, []models.Rule, error) {
	stepID := stepIDFromCtx(ctx)
	parts := []string{}

	if a.promptPaths != nil && a.fs != nil {
		corePromptFile := a.promptPaths.CorePrompt("system_prompt.md")
		if content, err := a.fs.ReadFile(corePromptFile); err == nil && strings.TrimSpace(string(content)) != "" {
			cStr := string(content)
			if !strings.HasPrefix(strings.TrimSpace(cStr), "# Base System Prompt") {
				cStr = "# Base System Prompt\n" + cStr
			}
			parts = append(parts, cStr)
		}
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

	// 1. Global Rules
	if len(globalRules) > 0 {
		filtered := filterRulesForAgent(globalRules, agent, stepID)
		if len(filtered) > 0 {
			parts = append(parts, "# Global Rules [IMMUTABLE - DO NOT OVERRIDE]\n"+formatRules(filtered))
		}
	}

	// 2. Agent Role Constraints
	if agent != nil && a.promptPaths != nil && a.fs != nil {
		rolePromptFile := a.promptPaths.RolePrompt(agent.Role)
		if content, err := a.fs.ReadFile(rolePromptFile); err == nil && strings.TrimSpace(string(content)) != "" {
			parts = append(parts, "# Agent Role Constraints\n"+string(content))
		}
	}

	// Step-specific Instructions
	if stepID != "" && a.promptPaths != nil && a.fs != nil {
		stepPromptFile := a.promptPaths.StepPrompt(stepID)
		if content, err := a.fs.ReadFile(stepPromptFile); err == nil && strings.TrimSpace(string(content)) != "" {
			parts = append(parts, "# Step-specific Instructions\n"+string(content))
		}
	}

	// 3. Project Rules
	if len(projectRules) > 0 {
		filtered := filterRulesForAgent(projectRules, agent, stepID)
		if len(filtered) > 0 {
			parts = append(parts, "# Project Rules\n"+formatRules(filtered))
		}
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
	if a.promptPaths != nil && a.fs != nil {
		outputRulesFile := a.promptPaths.CorePrompt("output_rules.md")
		if content, err := a.fs.ReadFile(outputRulesFile); err == nil && strings.TrimSpace(string(content)) != "" {
			parts = append(parts, string(content))
		}
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
