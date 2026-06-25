package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	orchestratorworkspace "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/retrieval"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepAnalyze(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepCtx workflow.StepContext) (map[string]any, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	ctx = context.WithValue(ctx, retrieval.WorkspaceRootKey, localPath)

	if o.prompts != nil {
		messages, tools, err := o.prompts.AssembleForAgent(ctx, *task, agent, nil)
		if err != nil {
			return nil, fmt.Errorf("assemble prompt: %w", err)
		}
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
	}
	if patch.TaskReadyForExecution(task) {
		return map[string]any{"complexity": task.Complexity, "spec_status": task.SpecStatus}, nil
	}

	analysis, fallbackUsed, err := o.runAnalyzeProcess(ctx, task, agent, stepCtx)
	if err != nil {
		return nil, err
	}

	if analysis.Complexity == "" {
		analysis.Complexity = models.TaskComplexityEasy
	}

	o.writeOpenSpecFiles(ctx, task, localPath, analysis)

	return o.applyAnalyzePolicy(ctx, task, agent, analysis, fallbackUsed)
}

func (o *Orchestrator) runAnalyzeProcess(ctx context.Context, task *models.Task, agent *models.Agent, stepCtx workflow.StepContext) (models.TaskAnalysis, bool, error) {
	if o.llm == nil {
		return deriveWorkflowAnalysis(task), true, nil
	}

	instruction := o.buildAnalyzeInstruction(stepCtx)
	messages, err := o.buildAnalyzeMessages(ctx, task, agent, instruction)
	if err != nil {
		return models.TaskAnalysis{}, false, err
	}

	parsedFinal, loopErr := o.runAnalyzeLLMLoop(ctx, task, agent, messages)
	if loopErr != nil || parsedFinal == nil {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("agent failed to output a final spec JSON: %v, falling back to derived analysis", loopErr))
		return deriveWorkflowAnalysis(task), true, nil
	}

	return o.parseAnalysisFinal(parsedFinal), false, nil
}

func (o *Orchestrator) buildAnalyzeMessages(ctx context.Context, task *models.Task, agent *models.Agent, instruction string) ([]llm.Message, error) {
	var messages []llm.Message
	var err error
	if o.prompts != nil {
		messages, _, err = o.prompts.AssembleForAgent(ctx, *task, agent, nil)
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

func (o *Orchestrator) buildAnalyzeInstruction(stepCtx workflow.StepContext) string {
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

func (o *Orchestrator) runAnalyzeLLMLoop(ctx context.Context, task *models.Task, agent *models.Agent, messages []llm.Message) (map[string]any, error) {
	maxIterations := 6
	analyzeTools := analyzeToolDefinitions()

	for i := 0; i < maxIterations; i++ {
		routeName := agent.ModelLevelGroup
		if o.projects != nil {
			if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
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

		resp, err := o.llm.ChatWithOptions(routeCtx, messages, llm.ChatOptions{Tools: analyzeTools, ToolChoice: "auto"})
		if err != nil {
			return nil, fmt.Errorf("llm tool loop call failed: %w", err)
		}
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("StepAnalyze Iteration %d: response from %s", i+1, resp.Model))

		if len(resp.ToolCalls) > 0 {
			o.writeLLMCallTrace(ctx, task, agent, workflow.StepAnalyze, messages, resp, map[string]any{"tool_calls": resp.ToolCalls})
			messages = append(messages, llm.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})
			for _, call := range resp.ToolCalls {
				toolResult := o.executeAnalyzeTool(ctx, task, agent, call.Name, call.Arguments)
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
			o.writeLLMCallTrace(ctx, task, agent, workflow.StepAnalyze, messages, resp, map[string]any{"raw_content": resp.Content})
			o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("StepAnalyze Iteration %d: output is invalid JSON: %v", i+1, parseErr))
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

		o.writeLLMCallTrace(ctx, task, agent, workflow.StepAnalyze, messages, resp, parsedJSON)

		if toolUse, ok := parsedJSON["tool_use"].(map[string]any); ok {
			toolName, _ := toolUse["name"].(string)
			toolArgs, _ := toolUse["arguments"].(map[string]any)
			argsBytes, _ := json.Marshal(toolArgs)
			o.log(ctx, task.ID, nil, "info", fmt.Sprintf("Agent requested legacy tool %s with args %v", toolName, toolArgs))
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool %s result:\n%s\n\nPlease output either the next native tool call or the final spec JSON.", toolName, o.executeAnalyzeTool(ctx, task, agent, toolName, string(argsBytes))),
			})
			continue
		}

		return parsedJSON, nil
	}

	return nil, fmt.Errorf("exceeded max iterations (%d)", maxIterations)
}

func (o *Orchestrator) parseAnalysisFinal(parsedFinal map[string]any) models.TaskAnalysis {
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

func (o *Orchestrator) writeOpenSpecFiles(ctx context.Context, task *models.Task, localPath string, analysis models.TaskAnalysis) {
	changeName := patch.DeriveChangeName(task)
	changeDir := filepath.Join(localPath, "openspec", "changes", changeName)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to create change directory: %v", err))
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
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save proposal.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "specs.md"), []byte(specsContent), 0o644); err != nil {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save specs.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "design.md"), []byte(designContent), 0o644); err != nil {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save design.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasksContent), 0o644); err != nil {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save tasks.md: %v", err))
	}

	meta := fmt.Sprintf("changeName: %s\ntaskId: %s\nstatus: pending_review\n", changeName, task.ID)
	if err := os.WriteFile(filepath.Join(changeDir, ".openspec.yaml"), []byte(meta), 0o644); err != nil {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save .openspec.yaml: %v", err))
	}
}

func (o *Orchestrator) applyAnalyzePolicy(ctx context.Context, task *models.Task, agent *models.Agent, analysis models.TaskAnalysis, fallbackUsed bool) (map[string]any, error) {
	oldComplexity := task.Complexity
	raw, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}

	var projectAutonomy string
	var projectReviewPolicy string
	if o.projects != nil {
		if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
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

	if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{
		Complexity: &analysis.Complexity,
		Analysis:   raw,
		SpecStatus: &specStatus,
	}); err != nil {
		return nil, fmt.Errorf("update task metadata: %w", err)
	}

	if _, err := o.updateTaskStatus(ctx, task.ID, status); err != nil {
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

func analyzeToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "list_files",
			Description: "List relevant source files in the task workspace. Use this before reading files when repository structure is unknown.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		},
		{
			Name:        "read_file",
			Description: "Read a single workspace file by repository-relative path.",
			Parameters:  json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string","description":"Repository-relative file path to read."}},"additionalProperties":false}`),
		},
		{
			Name:        "grep_search",
			Description: "Search workspace files for a literal query string and return matching lines.",
			Parameters:  json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string","description":"Literal text to search for."}},"additionalProperties":false}`),
		},
	}
}

func (o *Orchestrator) executeAnalyzeTool(ctx context.Context, task *models.Task, agent *models.Agent, toolName, arguments string) string {
	var args map[string]any
	if strings.TrimSpace(arguments) != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return fmt.Sprintf("Error: invalid tool arguments JSON: %v", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	switch toolName {
	case "list_files":
		result, err := o.listAnalyzeFiles(ctx, task, agent)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	case "read_file":
		path, _ := args["path"].(string)
		if strings.TrimSpace(path) == "" {
			return `Error: missing required "path" argument`
		}
		result, err := o.readAnalyzeFile(ctx, task, agent, path)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	case "grep_search":
		query, _ := args["query"].(string)
		if strings.TrimSpace(query) == "" {
			return `Error: missing required "query" argument`
		}
		result, err := o.grepAnalyzeFiles(ctx, task, agent, query)
		if err != nil {
			return "Error: " + err.Error()
		}
		return result
	default:
		return fmt.Sprintf("Error: unknown analyze tool %q", toolName)
	}
}

type analyzeSourceRoot struct {
	path   string
	prefix string
}

func (o *Orchestrator) analyzeSourceRoots(ctx context.Context, task *models.Task) []analyzeSourceRoot {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	ws, err := o.LoadTaskWorkspace(ctx, task)
	if err != nil || ws == nil || len(ws.Repos) == 0 {
		return []analyzeSourceRoot{{path: localPath}}
	}

	var roots []analyzeSourceRoot
	targetCount := 0
	for _, repo := range ws.Repos {
		if task.RepositoryID != nil && repo.RepoID != *task.RepositoryID {
			continue
		}
		if repo.Paths.Main == "" {
			continue
		}
		targetCount++
	}
	for _, repo := range ws.Repos {
		if task.RepositoryID != nil && repo.RepoID != *task.RepositoryID {
			continue
		}
		if repo.Paths.Main == "" {
			continue
		}
		prefix := ""
		if task.RepositoryID == nil && targetCount > 1 {
			prefix = repo.Name
		}
		roots = append(roots, analyzeSourceRoot{
			path:   filepath.Join(ws.Root, repo.Paths.Main),
			prefix: prefix,
		})
	}
	if len(roots) == 0 {
		return []analyzeSourceRoot{{path: localPath}}
	}
	return roots
}

func (o *Orchestrator) listAnalyzeFiles(ctx context.Context, task *models.Task, agent *models.Agent) (string, error) {
	var files []string
	for _, root := range o.analyzeSourceRoots(ctx, task) {
		containerRoot := o.containerPathForHostPath(task, root.path, "")
		cmd := fmt.Sprintf("cd %s && find . \\( -name .git -o -name node_modules -o -name vendor -o -name dist -o -name artifacts -o -name logs -o -name specs -o -name openspec -o -name context -o -name pr \\) -prune -o -type f -print | sed 's#^\\\\./##'", orchestratorworkspace.QuoteShellArg(containerRoot))
		out, err := o.runAnalyzeSandboxCommand(ctx, task, agent, cmd)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			files = append(files, filepath.ToSlash(filepath.Join(root.prefix, line)))
		}
	}
	if len(files) == 0 {
		return "No files found in workspace.", nil
	}
	return strings.Join(files, "\n"), nil
}

func (o *Orchestrator) readAnalyzeFile(ctx context.Context, task *models.Task, agent *models.Agent, subPath string) (string, error) {
	subPath = filepath.Clean(strings.TrimSpace(subPath))
	for _, root := range o.analyzeSourceRoots(ctx, task) {
		relPath := subPath
		if root.prefix != "" {
			prefix := root.prefix + string(filepath.Separator)
			if !strings.HasPrefix(subPath, prefix) {
				continue
			}
			relPath = strings.TrimPrefix(subPath, prefix)
		}
		if !orchestratorworkspace.IsSafeRelativeSourcePath(relPath) {
			continue
		}
		containerRoot := o.containerPathForHostPath(task, root.path, "")
		cmd := fmt.Sprintf("cd %s && if [ -f %s ]; then head -c 20000 %s; else exit 2; fi",
			orchestratorworkspace.QuoteShellArg(containerRoot),
			orchestratorworkspace.QuoteShellArg(relPath),
			orchestratorworkspace.QuoteShellArg(relPath),
		)
		content, err := o.runAnalyzeSandboxCommand(ctx, task, agent, cmd)
		if err == nil {
			return content, nil
		}
	}
	return "", fmt.Errorf("file %s not found in source roots", subPath)
}

func (o *Orchestrator) grepAnalyzeFiles(ctx context.Context, task *models.Task, agent *models.Agent, query string) (string, error) {
	var matches []string
	for _, root := range o.analyzeSourceRoots(ctx, task) {
		containerRoot := o.containerPathForHostPath(task, root.path, "")
		cmd := fmt.Sprintf("cd %s && grep -RIn --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=vendor --exclude-dir=dist --exclude-dir=artifacts --exclude-dir=logs --exclude-dir=specs --exclude-dir=openspec --exclude-dir=context --exclude-dir=pr -- %s . || true",
			orchestratorworkspace.QuoteShellArg(containerRoot),
			orchestratorworkspace.QuoteShellArg(query),
		)
		result, err := o.runAnalyzeSandboxCommand(ctx, task, agent, cmd)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(result, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "./") {
				line = strings.TrimPrefix(line, "./")
			}
			matches = append(matches, filepath.ToSlash(filepath.Join(root.prefix, line)))
		}
	}
	if len(matches) == 0 {
		return "No matches found.", nil
	}
	return strings.Join(matches, "\n"), nil
}

func (o *Orchestrator) runAnalyzeSandboxCommand(ctx context.Context, task *models.Task, agent *models.Agent, command string) (string, error) {
	if o.runtime == nil {
		return "", fmt.Errorf("sandbox runtime is not configured")
	}
	agentID := ""
	if agent != nil {
		agentID = agent.ID
	}
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agentID,
		Workspace:   localPath,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     time.Minute,
	})
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("analyze sandbox command failed with exit code %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return result.Stdout, nil
}
