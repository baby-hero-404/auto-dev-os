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


// AnalyzeStep implements Step for the analysis phase.
type AnalyzeStep struct {
	rt            StepRuntime
	workspaceRoot string
	tasks         TaskReader
	taskUpdate    TaskUpdater
	projects      ProjectReader
	llm           LLMChatter
	prompts       PromptAssembler
	sandbox       SandboxRunner
	artifacts     ArtifactSaver
	status        StatusUpdater
	traces        TraceRecorder
	log           Logger
	wkspace       WorkspaceLoader
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string
}

func NewAnalyzeStep(
	rt StepRuntime,
	workspaceRoot string,
	tasks TaskReader,
	taskUpdate TaskUpdater,
	projects ProjectReader,
	llm LLMChatter,
	prompts PromptAssembler,
	sandbox SandboxRunner,
	artifacts ArtifactSaver,
	status StatusUpdater,
	traces TraceRecorder,
	log Logger,
	wkspace WorkspaceLoader,
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string,
) *AnalyzeStep {
	return &AnalyzeStep{
		rt:            rt,
		workspaceRoot: workspaceRoot,
		tasks:         tasks,
		taskUpdate:    taskUpdate,
		projects:      projects,
		llm:           llm,
		prompts:       prompts,
		sandbox:       sandbox,
		artifacts:     artifacts,
		status:        status,
		traces:        traces,
		log:           log,
		wkspace:       wkspace,
		containerPath: containerPath,
	}
}

func (s *AnalyzeStep) ID() string                              { return workflow.StepAnalyze }
func (s *AnalyzeStep) StatusOnResume(_ StepResult) string        { return models.TaskStatusAnalyzing }

func (s *AnalyzeStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	localPath := sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID)
	ctx = context.WithValue(ctx, retrieval.WorkspaceRootKey, localPath)

	if s.prompts != nil {
		messages, tools, err := s.prompts.AssembleForAgent(ctx, *s.rt.Task, s.rt.Agent, nil)
		if err != nil {
			return nil, fmt.Errorf("assemble prompt: %w", err)
		}
		s.log.Log(ctx, s.rt.Task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
	}
	if patch.TaskReadyForExecution(s.rt.Task) {
		return StepResult{"complexity": s.rt.Task.Complexity, "spec_status": s.rt.Task.SpecStatus}, nil
	}

	analysis, fallbackUsed, err := s.runAnalyzeProcess(ctx, stepCtx)
	if err != nil {
		return nil, err
	}

	if analysis.Complexity == "" {
		analysis.Complexity = models.TaskComplexityEasy
	}

	s.writeOpenSpecFiles(ctx, localPath, analysis)

	return s.applyAnalyzePolicy(ctx, analysis, fallbackUsed)
}

func (s *AnalyzeStep) runAnalyzeProcess(ctx context.Context, stepCtx workflow.StepContext) (models.TaskAnalysis, bool, error) {
	if s.llm == nil {
		return deriveWorkflowAnalysis(s.rt.Task), true, nil
	}

	instruction := buildAnalyzeInstruction(stepCtx)
	messages, err := s.buildAnalyzeMessages(ctx, instruction)
	if err != nil {
		return models.TaskAnalysis{}, false, err
	}

	parsedFinal, loopErr := s.runAnalyzeLLMLoop(ctx, messages)
	if loopErr != nil || parsedFinal == nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("agent failed to output a final spec JSON: %v, falling back to derived analysis", loopErr))
		return deriveWorkflowAnalysis(s.rt.Task), true, nil
	}

	return parseAnalysisFinal(parsedFinal), false, nil
}

func (s *AnalyzeStep) buildAnalyzeMessages(ctx context.Context, instruction string) ([]llm.Message, error) {
	var messages []llm.Message
	var err error
	if s.prompts != nil {
		messages, _, err = s.prompts.AssembleForAgent(ctx, *s.rt.Task, s.rt.Agent, nil)
		if err != nil {
			return nil, err
		}
	} else {
		messages = []llm.Message{{Role: "user", Content: s.rt.Task.Title + "\n\n" + s.rt.Task.Description}}
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

func (s *AnalyzeStep) runAnalyzeLLMLoop(ctx context.Context, messages []llm.Message) (map[string]any, error) {
	maxIterations := 6
	analyzeTools := analyzeToolDefinitions()

	for i := 0; i < maxIterations; i++ {
		routeName := s.rt.Agent.ModelLevelGroup
		if s.projects != nil {
			if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
				if s.rt.Agent.Role == models.AgentRolePlanner && p.DefaultModelLevel != "" {
					routeName = p.DefaultModelLevel
				} else if (routeName == "" || routeName == "default") && p.DefaultModelLevel != "" {
					routeName = p.DefaultModelLevel
				}
			}
		}
		routeCtx := llm.WithRouteOptions(ctx, llm.RouteOptions{
			Complexity: s.rt.Task.Complexity,
			OrgID:      s.rt.Agent.OrgID,
			ProjectID:  s.rt.Task.ProjectID,
			AgentID:    s.rt.Agent.ID,
			TaskID:     s.rt.Task.ID,
			RouteName:  routeName,
		})

		resp, err := s.llm.ChatWithOptions(routeCtx, messages, llm.ChatOptions{Tools: analyzeTools, ToolChoice: "auto"})
		if err != nil {
			return nil, fmt.Errorf("llm tool loop call failed: %w", err)
		}
		s.log.Log(ctx, s.rt.Task.ID, nil, "info", fmt.Sprintf("StepAnalyze Iteration %d: response from %s", i+1, resp.Model))

		if len(resp.ToolCalls) > 0 {
			if s.traces != nil {
				s.traces.WriteLLMCallTrace(ctx, s.rt.Task, s.rt.Agent, workflow.StepAnalyze, messages, resp, map[string]any{"tool_calls": resp.ToolCalls})
			}
			messages = append(messages, llm.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})
			for _, call := range resp.ToolCalls {
				toolResult := s.executeAnalyzeTool(ctx, call.Name, call.Arguments)
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
			if s.traces != nil {
				s.traces.WriteLLMCallTrace(ctx, s.rt.Task, s.rt.Agent, workflow.StepAnalyze, messages, resp, map[string]any{"raw_content": resp.Content})
			}
			s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("StepAnalyze Iteration %d: output is invalid JSON: %v", i+1, parseErr))
			content := resp.Content
			if content == "" {
				content = "(empty response)"
			}
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: content,
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Your output was not valid JSON. Error: %v. Please correct the formatting/syntax and output strictly valid JSON matching the schema.", parseErr),
			})
			continue
		}

		if s.traces != nil {
			s.traces.WriteLLMCallTrace(ctx, s.rt.Task, s.rt.Agent, workflow.StepAnalyze, messages, resp, parsedJSON)
		}

		if toolUse, ok := parsedJSON["tool_use"].(map[string]any); ok {
			toolName, _ := toolUse["name"].(string)
			toolArgs, _ := toolUse["arguments"].(map[string]any)
			argsBytes, _ := json.Marshal(toolArgs)
			s.log.Log(ctx, s.rt.Task.ID, nil, "info", fmt.Sprintf("Agent requested legacy tool %s with args %v", toolName, toolArgs))
			content := resp.Content
			if content == "" {
				content = "(empty response)"
			}
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: content,
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool %s result:\n%s\n\nPlease output either the next native tool call or the final spec JSON.", toolName, s.executeAnalyzeTool(ctx, toolName, string(argsBytes))),
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

func (s *AnalyzeStep) writeOpenSpecFiles(ctx context.Context, localPath string, analysis models.TaskAnalysis) {
	changeName := patch.DeriveChangeName(s.rt.Task)
	changeDir := filepath.Join(localPath, "openspec", "changes", changeName)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to create change directory: %v", err))
		return
	}

	proposalContent := analysis.ProposalMD
	if proposalContent == "" {
		proposalContent = fmt.Sprintf("## Proposal for %s\n\n%s\n", s.rt.Task.Title, s.rt.Task.Description)
	}
	specsContent := analysis.SpecsMD
	if specsContent == "" {
		specsContent = fmt.Sprintf("## ADDED Requirements\n\n### Requirement: %s\n%s\n", s.rt.Task.Title, s.rt.Task.Description)
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
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save proposal.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "specs.md"), []byte(specsContent), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save specs.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "design.md"), []byte(designContent), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save design.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasksContent), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save tasks.md: %v", err))
	}

	meta := fmt.Sprintf("changeName: %s\ntaskId: %s\nstatus: pending_review\n", changeName, s.rt.Task.ID)
	if err := os.WriteFile(filepath.Join(changeDir, ".openspec.yaml"), []byte(meta), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save .openspec.yaml: %v", err))
	}
}

func (s *AnalyzeStep) applyAnalyzePolicy(ctx context.Context, analysis models.TaskAnalysis, fallbackUsed bool) (StepResult, error) {
	oldComplexity := s.rt.Task.Complexity
	raw, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}

	var projectAutonomy string
	var projectReviewPolicy string
	if s.projects != nil {
		if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
			projectAutonomy = p.DefaultAutonomy
			projectReviewPolicy = p.AutoReviewPolicy
		}
	}

	specStatus, status := policy.ShouldAutoApproveSpec(
		analysis.Complexity,
		analysis.AffectedFiles,
		analysis.RiskDomains,
		s.rt.Agent.AutonomyLevel,
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

	if s.taskUpdate != nil {
		if _, err := s.taskUpdate.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{
			Complexity: &analysis.Complexity,
			Analysis:   raw,
			SpecStatus: &specStatus,
		}); err != nil {
			return nil, fmt.Errorf("update task metadata: %w", err)
		}
	}

	if s.status != nil {
		if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, status); err != nil {
			return nil, fmt.Errorf("update task status: %w", err)
		}
	}

	s.rt.Task.Complexity = analysis.Complexity
	s.rt.Task.SpecStatus = specStatus
	s.rt.Task.Analysis = raw

	if specStatus == models.TaskSpecStatusPendingReview || specStatus == models.TaskSpecStatusChangesRequested {
		return nil, workflow.PauseError{Step: workflow.StepAnalyze, Reason: pauseReason}
	}

	if oldComplexity != analysis.Complexity && specStatus == models.TaskSpecStatusAutoApproved {
		return StepResult{"complexity": analysis.Complexity, "spec_status": specStatus}, workflow.ErrGraphChanged
	}

	return StepResult{"complexity": analysis.Complexity, "spec_status": specStatus}, nil
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

