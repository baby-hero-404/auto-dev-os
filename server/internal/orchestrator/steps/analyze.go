package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/retrieval"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecuteAnalyze(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, stepCtx workflow.StepContext) (map[string]any, error) {
	localPath := sandbox.WorkspacePath(deps.WorkspaceRoot, task.ID)
	ctx = context.WithValue(ctx, retrieval.WorkspaceRootKey, localPath)

	if deps.Prompts != nil {
		messages, tools, err := deps.Prompts.AssembleForAgent(ctx, *task, agent, nil)
		if err != nil {
			return nil, fmt.Errorf("assemble prompt: %w", err)
		}
		deps.Log(ctx, task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
	}
	if patch.TaskReadyForExecution(task) {
		return map[string]any{"complexity": task.Complexity, "spec_status": task.SpecStatus}, nil
	}

	analysis, fallbackUsed, err := runAnalyzeProcess(ctx, deps, task, agent, stepCtx)
	if err != nil {
		return nil, err
	}

	if analysis.Complexity == "" {
		analysis.Complexity = models.TaskComplexityEasy
	}

	writeOpenSpecFiles(ctx, deps, task, localPath, analysis)

	return applyAnalyzePolicy(ctx, deps, task, agent, analysis, fallbackUsed)
}

func runAnalyzeProcess(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, stepCtx workflow.StepContext) (models.TaskAnalysis, bool, error) {
	if deps.LLM == nil {
		return deriveWorkflowAnalysis(task), true, nil
	}

	instruction := buildAnalyzeInstruction(stepCtx)
	messages, err := buildAnalyzeMessages(ctx, deps, task, agent, instruction)
	if err != nil {
		return models.TaskAnalysis{}, false, err
	}

	parsedFinal, loopErr := runAnalyzeLLMLoop(ctx, deps, task, agent, messages)
	if loopErr != nil || parsedFinal == nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("agent failed to output a final spec JSON: %v, falling back to derived analysis", loopErr))
		return deriveWorkflowAnalysis(task), true, nil
	}

	return parseAnalysisFinal(parsedFinal), false, nil
}

func buildAnalyzeMessages(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, instruction string) ([]llm.Message, error) {
	var messages []llm.Message
	var err error
	if deps.Prompts != nil {
		messages, _, err = deps.Prompts.AssembleForAgent(ctx, *task, agent, nil)
		if err != nil {
			return nil, err
		}
	} else {
		messages = []llm.Message{{Role: "user", Content: task.Title + "\n\n" + task.Description}}
	}
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: "Workflow step: " + workflow.StepAnalyze + "\n\n" + instruction,
	})
	return messages, nil
}

func buildAnalyzeInstruction(stepCtx workflow.StepContext) string {
	instruction := `Analyze this task and output the proposed specification as a valid JSON object.
You have access to read-only native tools to retrieve more context about the workspace files before writing your final specification.

If you do not have enough context about the current workspace files (or if the task description is generic/vague), you MUST use the "list_files" tool first to inspect the repository structure, and then "read_file" to read the relevant source files before writing your specification or generating questions.
Once you have gathered enough information and are ready to provide the final specification, output the final specification JSON matching the expected format.

You must output ONLY a valid JSON object (or inside a ` + "```json" + ` block).
The JSON object MUST have the following structure:
{
  "complexity": "easy" | "medium" | "hard",
  "primary_category": "frontend" | "backend" | "database" | "devops" | "qa" | "security",
  "scope": "A clear, detailed description of the scope of the change",
  "affected_files": ["list", "of", "files", "expected", "to", "be", "modified"],
  "risks": ["list", "of", "potential", "risks", "and", "challenges"],
  "risk_domains": ["list", "of", "risk", "domains", "touched", "(e.g., 'auth', 'payment', 'security', 'data_migration', 'infra', 'rbac', 'public_api')"],
  "execution_plan": ["step-by-step", "plan", "to", "implement", "this", "task"],
  "clarification_questions": ["questions", "if", "more", "details", "are", "needed"],
  "required_skills": ["list", "of", "skill", "names", "required", "for", "this", "task", "(e.g., 'docker_expert', 'frontend_design')"],
  "proposal_md": "Markdown for proposal.md (use the template below)",
  "specs_md": "Markdown for specs.md (use the template below)",
  "design_md": "Markdown for design.md (use the template below)",
  "tasks_md": "Markdown for tasks.md (use the template below)"
}

=== OPENSPEC TEMPLATE: proposal.md ===
## Why
(1-2 sentences: what problem does this solve? Why now?)

## What Changes
(Bullet list of specific changes. Mark breaking changes with **BREAKING**.)

## Capabilities
### New Capabilities
- ` + "`<name>`" + `: <brief description>

### Modified Capabilities
- ` + "`<existing-name>`" + `: <what requirement is changing>

## Impact
(Affected code, APIs, dependencies, systems)

=== OPENSPEC TEMPLATE: specs.md ===
Use delta operations as section headers:
## ADDED Requirements
### Requirement: <name>
<Description using SHALL/MUST language>

#### Scenario: <scenario name>
- **WHEN** <condition>
- **THEN** <expected outcome>

## MODIFIED Requirements
(Same format, include full updated content)

## REMOVED Requirements
### Requirement: <name>
**Reason**: <why removed>
**Migration**: <how to migrate>

=== OPENSPEC TEMPLATE: design.md ===
## Context
(Background, current state, constraints)

## Goals / Non-Goals
**Goals:** ...
**Non-Goals:** ...

## Decisions
(Key technical choices with rationale)

## Risks / Trade-offs
(Known limitations, format: [Risk] → Mitigation)

## Open Questions
(Outstanding decisions or unknowns)

=== OPENSPEC TEMPLATE: tasks.md ===
Group related tasks under numbered headings. Each task MUST be a checkbox.
## 1. <Group Name>
- [ ] 1.1 <Task description>
- [ ] 1.2 <Task description>

## 2. <Group Name>
- [ ] 2.1 <Task description>
`

	var repoContext string
	if contextOut, ok := stepCtx.Inputs[workflow.StepContextLoad]; ok {
		if contextJSON, err := json.Marshal(contextOut); err == nil {
			repoContext = string(contextJSON)
		}
	}
	if repoContext != "" {
		instruction += "\n\n=== UNTRUSTED REPOSITORY-CONTROLLED CONTEXT (potentially outdated or invalid) ===\n" + repoContext
	}
	return instruction
}

func runAnalyzeLLMLoop(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, messages []llm.Message) (map[string]any, error) {
	maxIterations := 6
	analyzeTools := analyzeToolDefinitions()

	for i := 0; i < maxIterations; i++ {
		routeName := agent.ModelLevelGroup
		if deps.Projects != nil {
			if p, err := deps.Projects.GetByID(ctx, task.ProjectID); err == nil {
				if agent.Role == models.AgentRolePlanner && p.DefaultModelLevel != "" {
					routeName = p.DefaultModelLevel
				} else if (routeName == "" || routeName == "default") && p.DefaultModelLevel != "" {
					routeName = p.DefaultModelLevel
				}
			}
		}
		routeCtx := llm.WithRouteOptions(ctx, llm.RouteOptions{
			Complexity: task.Complexity,
			OrgID:      agent.OrgID,
			ProjectID:  task.ProjectID,
			AgentID:    agent.ID,
			TaskID:     task.ID,
			RouteName:  routeName,
		})

		resp, err := deps.LLM.ChatWithOptions(routeCtx, messages, llm.ChatOptions{Tools: analyzeTools, ToolChoice: "auto"})
		if err != nil {
			return nil, fmt.Errorf("llm tool loop call failed: %w", err)
		}
		deps.Log(ctx, task.ID, nil, "info", fmt.Sprintf("StepAnalyze Iteration %d: response from %s", i+1, resp.Model))

		if len(resp.ToolCalls) > 0 {
			deps.WriteLLMCallTrace(ctx, task, agent, workflow.StepAnalyze, messages, resp, map[string]any{"tool_calls": resp.ToolCalls})
			messages = append(messages, llm.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})
			for _, call := range resp.ToolCalls {
				toolResult := executeAnalyzeTool(ctx, deps, task, agent, call.Name, call.Arguments)
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: call.ID,
					ToolName:   call.Name,
					Content:    toolResult,
				})
			}
			continue
		}

		parsedJSON, parseErr := llmrunner.ParseJSONMarkdown(resp.Content)
		if parseErr != nil {
			deps.WriteLLMCallTrace(ctx, task, agent, workflow.StepAnalyze, messages, resp, map[string]any{"raw_content": resp.Content})
			deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("StepAnalyze Iteration %d: output is invalid JSON: %v", i+1, parseErr))
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Your output was not valid JSON. Error: %v. Please correct the formatting/syntax and output strictly valid JSON matching the schema.", parseErr),
			})
			continue
		}

		deps.WriteLLMCallTrace(ctx, task, agent, workflow.StepAnalyze, messages, resp, parsedJSON)

		if toolUse, ok := parsedJSON["tool_use"].(map[string]any); ok {
			toolName, _ := toolUse["name"].(string)
			toolArgs, _ := toolUse["arguments"].(map[string]any)
			argsBytes, _ := json.Marshal(toolArgs)
			deps.Log(ctx, task.ID, nil, "info", fmt.Sprintf("Agent requested legacy tool %s with args %v", toolName, toolArgs))
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool %s result:\n%s\n\nPlease output either the next native tool call or the final spec JSON.", toolName, executeAnalyzeTool(ctx, deps, task, agent, toolName, string(argsBytes))),
			})
			continue
		}

		return parsedJSON, nil
	}

	return nil, fmt.Errorf("exceeded max iterations (%d)", maxIterations)
}

func parseAnalysisFinal(parsedFinal map[string]any) models.TaskAnalysis {
	var analysis models.TaskAnalysis
	if comp, ok := parsedFinal["complexity"].(string); ok {
		analysis.Complexity = comp
	}
	if cat, ok := parsedFinal["primary_category"].(string); ok {
		analysis.PrimaryCategory = cat
	}
	if scope, ok := parsedFinal["scope"].(string); ok {
		analysis.Scope = scope
	}
	if aff, ok := parsedFinal["affected_files"].([]any); ok {
		for _, item := range aff {
			if s, ok := item.(string); ok {
				analysis.AffectedFiles = append(analysis.AffectedFiles, s)
			}
		}
	}
	if risks, ok := parsedFinal["risks"].([]any); ok {
		for _, item := range risks {
			if s, ok := item.(string); ok {
				analysis.Risks = append(analysis.Risks, s)
			}
		}
	}
	if execPlan, ok := parsedFinal["execution_plan"].([]any); ok {
		for _, item := range execPlan {
			if s, ok := item.(string); ok {
				analysis.ExecutionPlan = append(analysis.ExecutionPlan, s)
			}
		}
	}
	if questions, ok := parsedFinal["clarification_questions"].([]any); ok {
		for _, item := range questions {
			if s, ok := item.(string); ok {
				analysis.ClarificationQuestions = append(analysis.ClarificationQuestions, s)
			}
		}
	}
	if skills, ok := parsedFinal["required_skills"].([]any); ok {
		for _, item := range skills {
			if s, ok := item.(string); ok {
				analysis.RequiredSkills = append(analysis.RequiredSkills, s)
			}
		}
	}
	if domains, ok := parsedFinal["risk_domains"].([]any); ok {
		for _, item := range domains {
			if s, ok := item.(string); ok {
				analysis.RiskDomains = append(analysis.RiskDomains, s)
			}
		}
	}
	if proposal, ok := parsedFinal["proposal_md"].(string); ok {
		analysis.ProposalMD = proposal
	}
	if specs, ok := parsedFinal["specs_md"].(string); ok {
		analysis.SpecsMD = specs
	}
	if design, ok := parsedFinal["design_md"].(string); ok {
		analysis.DesignMD = design
	}
	if tasks, ok := parsedFinal["tasks_md"].(string); ok {
		analysis.TasksMD = tasks
	}
	return analysis
}

func writeOpenSpecFiles(ctx context.Context, deps *Deps, task *models.Task, localPath string, analysis models.TaskAnalysis) {
	changeName := patch.DeriveChangeName(task)
	changeDir := filepath.Join(localPath, "openspec", "changes", changeName)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to create change directory: %v", err))
		return
	}

	proposalContent := analysis.ProposalMD
	if proposalContent == "" {
		proposalContent = fmt.Sprintf("## Proposal for %s\n\n%s\n", task.Title, task.Description)
	}
	specsContent := analysis.SpecsMD
	if specsContent == "" {
		specsContent = fmt.Sprintf("## ADDED Requirements\n\n### Requirement: %s\n%s\n", task.Title, task.Description)
	}
	designContent := analysis.DesignMD
	if designContent == "" {
		designContent = "## Design\n\nImplementation design details.\n"
	}
	tasksContent := analysis.TasksMD
	if tasksContent == "" {
		var builder strings.Builder
		builder.WriteString("## Tasks\n\n")
		if len(analysis.ExecutionPlan) > 0 {
			for _, step := range analysis.ExecutionPlan {
				builder.WriteString(fmt.Sprintf("- [ ] %s\n", step))
			}
		} else {
			builder.WriteString("- [ ] Implement changes\n")
		}
		tasksContent = builder.String()
	}

	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte(proposalContent), 0o644); err != nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save proposal.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "specs.md"), []byte(specsContent), 0o644); err != nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save specs.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "design.md"), []byte(designContent), 0o644); err != nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save design.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasksContent), 0o644); err != nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save tasks.md: %v", err))
	}

	meta := fmt.Sprintf("changeName: %s\ntaskId: %s\nstatus: pending_review\n", changeName, task.ID)
	if err := os.WriteFile(filepath.Join(changeDir, ".openspec.yaml"), []byte(meta), 0o644); err != nil {
		deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save .openspec.yaml: %v", err))
	}
}

func applyAnalyzePolicy(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, analysis models.TaskAnalysis, fallbackUsed bool) (map[string]any, error) {
	oldComplexity := task.Complexity
	raw, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}

	var projectAutonomy string
	var projectReviewPolicy string
	if deps.Projects != nil {
		if p, err := deps.Projects.GetByID(ctx, task.ProjectID); err == nil {
			projectAutonomy = p.DefaultAutonomy
			projectReviewPolicy = p.AutoReviewPolicy
		}
	}

	specStatus, status := policy.ShouldAutoApproveSpec(
		analysis.Complexity,
		analysis.AffectedFiles,
		analysis.RiskDomains,
		agent.AutonomyLevel,
		projectAutonomy,
		projectReviewPolicy,
		len(analysis.ClarificationQuestions) > 0,
	)

	pauseReason := "workflow paused for human spec review"
	if fallbackUsed {
		specStatus = models.TaskSpecStatusPendingReview
		status = models.TaskStatusSpecReview
		pauseReason = "workflow paused for human spec review due to fallback from malformed analyzer output"
	}

	if _, err := deps.Tasks.Update(ctx, task.ID, models.UpdateTaskInput{
		Complexity: &analysis.Complexity,
		Analysis:   raw,
		SpecStatus: &specStatus,
	}); err != nil {
		return nil, fmt.Errorf("update task metadata: %w", err)
	}

	if _, err := deps.UpdateTaskStatus(ctx, task.ID, status); err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	task.Complexity = analysis.Complexity
	task.SpecStatus = specStatus
	task.Analysis = raw

	if specStatus == models.TaskSpecStatusPendingReview || specStatus == models.TaskSpecStatusChangesRequested {
		return nil, workflow.PauseError{Step: workflow.StepAnalyze, Reason: pauseReason}
	}

	if oldComplexity != analysis.Complexity && specStatus == models.TaskSpecStatusAutoApproved {
		return map[string]any{"complexity": analysis.Complexity, "spec_status": specStatus}, workflow.ErrGraphChanged
	}

	return map[string]any{"complexity": analysis.Complexity, "spec_status": specStatus}, nil
}

func deriveWorkflowAnalysis(task *models.Task) models.TaskAnalysis {
	text := strings.ToLower(task.Title + " " + task.Description)
	complexity := task.Complexity
	if complexity == "" {
		complexity = models.TaskComplexityEasy
	}
	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}
	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = models.TaskComplexityHard
			break
		}
	}
	if complexity != models.TaskComplexityHard {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = models.TaskComplexityMedium
				break
			}
		}
	}
	questions := []string{}
	if len(strings.TrimSpace(task.Description)) < 30 {
		questions = append(questions, "Please provide more implementation context, affected module names, and expected behavior.")
	}
	return models.TaskAnalysis{
		Complexity:    complexity,
		Scope:         "Generated by the Phase 3b workflow analyze step.",
		AffectedFiles: []string{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPlan: []string{
			"Assemble prompt with role, rules, and retrieved context.",
			"Decompose work into typed subtasks.",
			"Run backend and frontend coding tracks in parallel sandboxes.",
			"Merge, review, fix, test, and prepare PR approval checkpoint.",
		},
		ClarificationQuestions: questions,
	}
}
