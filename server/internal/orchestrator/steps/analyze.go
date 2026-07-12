package steps

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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
	maxCost       float64
	registry      *tool.Registry
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
	maxCost float64,
	registry *tool.Registry,
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
		maxCost:       maxCost,
		registry:      registry,
	}
}

func (s *AnalyzeStep) ID() string                         { return workflow.StepAnalyze }
func (s *AnalyzeStep) StatusOnResume(_ StepResult) string { return models.TaskStatusAnalyzing }

func (s *AnalyzeStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	localPath := sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID)
	ctx = context.WithValue(ctx, provider.WorkspaceRootKey, localPath)

	if s.prompts != nil {
		var plannerTools []llm.ToolDefinition
		if s.registry != nil {
			profile := tool.DefaultRoleProfiles()["planner"]
			plannerTools = s.registry.ToolsForCapabilities(profile.Capabilities)
		}
		
		plannerAgent := *s.rt.Agent
		plannerAgent.Role = models.AgentRolePlanner
		
		messages, tools, err := s.prompts.AssembleForAgent(ctx, *s.rt.Task, &plannerAgent, nil, plannerTools)
		if err != nil {
			return nil, fmt.Errorf("assemble prompt: %w", err)
		}
		s.log.Log(ctx, s.rt.Task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
	}
	if patch.TaskReadyForExecution(s.rt.Task) {
		if s.status != nil {
			currentStatus := s.rt.Task.Status
			if s.tasks != nil {
				if latest, err := s.tasks.GetByID(ctx, s.rt.Task.ID); err == nil && latest != nil {
					currentStatus = latest.Status
				}
			}
			if currentStatus == models.TaskStatusContextLoading || currentStatus == models.TaskStatusAnalyzing || currentStatus == models.TaskStatusSpecReview {
				if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusCoding); err != nil {
					return nil, fmt.Errorf("update task status: %w", err)
				}
				s.rt.Task.Status = models.TaskStatusCoding
			}
		}
		return StepResult{"complexity": s.rt.Task.Complexity, "spec_status": s.rt.Task.SpecStatus}, nil
	}

	analysis, fallbackUsed, err := s.runAnalyzeProcess(ctx, stepCtx)
	if err != nil {
		return nil, err
	}

	if analysis.Complexity == "" {
		analysis.Complexity = models.TaskComplexityEasy
	}

	s.writeOpenSpecFiles(ctx, localPath, &analysis)

	return s.applyAnalyzePolicy(ctx, analysis, fallbackUsed)
}

func (s *AnalyzeStep) runAnalyzeProcess(ctx context.Context, stepCtx workflow.StepContext) (models.TaskAnalysis, bool, error) {
	if s.llm == nil {
		return deriveWorkflowAnalysis(s.rt.Task), true, nil
	}

	instruction := s.buildAnalyzeInstruction(ctx, stepCtx)
	messages, err := s.buildAnalyzeMessages(ctx, instruction)
	if err != nil {
		return models.TaskAnalysis{}, false, err
	}

	parsedFinal, loopErr := s.runAnalyzeLLMLoop(ctx, messages)
	if loopErr != nil || parsedFinal == nil {
		if loopErr == nil {
			loopErr = fmt.Errorf("LLM returned empty or nil spec JSON")
		}
		return models.TaskAnalysis{}, false, fmt.Errorf("failed to generate valid specification after retry iterations: %w", loopErr)
	}

	return parseAnalysisFinal(parsedFinal), false, nil
}

func (s *AnalyzeStep) buildAnalyzeMessages(ctx context.Context, instruction string) ([]llm.Message, error) {
	var messages []llm.Message
	var err error
	if s.prompts != nil {
		stepCtx := context.WithValue(ctx, prompts.StepIDCtxKey, workflow.StepAnalyze)
		var plannerTools []llm.ToolDefinition
		if s.registry != nil {
			profile := tool.DefaultRoleProfiles()["planner"]
			plannerTools = s.registry.ToolsForCapabilities(profile.Capabilities)
		}
		
		plannerAgent := *s.rt.Agent
		plannerAgent.Role = models.AgentRolePlanner
		
		messages, _, err = s.prompts.AssembleForAgent(stepCtx, *s.rt.Task, &plannerAgent, nil, plannerTools)
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

type analyzeTemplateData struct {
	AvailableSkills []llm.ToolDefinition
	RepoContext    string
	WorkspaceFiles string
}

func (s *AnalyzeStep) buildAnalyzeInstruction(ctx context.Context, stepCtx workflow.StepContext) string {
	data := analyzeTemplateData{}
	
	if s.prompts != nil {
		if skills, err := s.prompts.ListAllSkills(ctx, *s.rt.Task); err == nil {
			data.AvailableSkills = skills
		}
	}

	if contextOut, ok := stepCtx.Inputs[workflow.StepContextLoad]; ok {
		data.RepoContext = formatRepoContext(contextOut)
	}
	if files, err := s.listAnalyzeFiles(ctx); err == nil && files != "" && files != "No files found in workspace." {
		data.WorkspaceFiles = files
	}

	tmplPath := filepath.Join("internal", "prompts", "templates", "analyze_instruction.tmpl")
	tmplBytes, err := os.ReadFile(tmplPath)
	if err != nil {
		// Fallback for unit tests
		tmplPath = filepath.Join("..", "..", "prompts", "templates", "analyze_instruction.tmpl")
		tmplBytes, err = os.ReadFile(tmplPath)
	}
	if err == nil {
		tmpl, err := template.New("analyze").Parse(string(tmplBytes))
		if err == nil {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err == nil {
				return buf.String()
			}
		}
	}

	// Fallback
	instruction := "Analyze this task and output the proposed specification as a valid JSON object matching the schema and template requested in the system instructions."
	instruction += "\nCRITICAL: If all requirements are clear and you have no NEW questions, you MUST return an empty array `[]` for `clarification_questions`. DO NOT repeat questions that have already been answered in the context."
	if data.RepoContext != "" {
		instruction += "\n\n=== UNTRUSTED REPOSITORY-CONTROLLED CONTEXT (potentially outdated or invalid) ===\n" + data.RepoContext
	}
	if data.WorkspaceFiles != "" {
		instruction += "\n\n=== Workspace Files ===\n" + data.WorkspaceFiles
	}
	return instruction
}

func (s *AnalyzeStep) runAnalyzeLLMLoop(ctx context.Context, messages []llm.Message) (map[string]any, error) {
	maxIterations := 6
	analyzeTools := s.analyzeToolDefinitions()

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

		callStart := time.Now()
		resp, err := s.llm.ChatWithOptions(routeCtx, messages, llm.ChatOptions{Tools: analyzeTools, ToolChoice: "auto"})
		callLatency := time.Since(callStart)
		if err != nil {
			return nil, fmt.Errorf("llm tool loop call failed: %w", err)
		}
		s.log.Log(ctx, s.rt.Task.ID, nil, "info", fmt.Sprintf("StepAnalyze Iteration %d: response from %s", i+1, resp.Model))

		if len(resp.ToolCalls) > 0 {
			if s.traces != nil {
				s.traces.WriteLLMCallTrace(ctx, s.rt.Task, s.rt.Agent, workflow.StepAnalyze, messages, resp, map[string]any{"tool_calls": resp.ToolCalls}, i+1, callLatency)
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
				s.traces.WriteLLMCallTrace(ctx, s.rt.Task, s.rt.Agent, workflow.StepAnalyze, messages, resp, map[string]any{"raw_content": resp.Content}, i+1, callLatency)
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
			s.traces.WriteLLMCallTrace(ctx, s.rt.Task, s.rt.Agent, workflow.StepAnalyze, messages, resp, parsedJSON, i+1, callLatency)
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

		// Contract Validation
		var missingFields []string
		if _, ok := parsedJSON["complexity"].(string); !ok {
			missingFields = append(missingFields, "complexity")
		}
		if _, ok := parsedJSON["primary_category"].(string); !ok {
			missingFields = append(missingFields, "primary_category")
		}
		if _, ok := parsedJSON["execution_phases"].([]any); !ok {
			missingFields = append(missingFields, "execution_phases")
		}
		if _, ok := parsedJSON["affected_files"].([]any); !ok {
			missingFields = append(missingFields, "affected_files")
		}
		if _, ok := parsedJSON["acceptance_criteria"].([]any); !ok {
			missingFields = append(missingFields, "acceptance_criteria")
		}
		_, hasBoundariesArray := parsedJSON["execution_boundaries"].([]any)
		_, hasBoundariesMap := parsedJSON["execution_boundaries"].(map[string]any)
		if !hasBoundariesArray && !hasBoundariesMap {
			missingFields = append(missingFields, "execution_boundaries")
		}
		if _, ok := parsedJSON["proposal_md"].(string); !ok {
			missingFields = append(missingFields, "proposal_md")
		}
		if _, ok := parsedJSON["specs_md"].(string); !ok {
			missingFields = append(missingFields, "specs_md")
		}
		if _, ok := parsedJSON["execution_units"].([]any); !ok {
			missingFields = append(missingFields, "execution_units")
		}
		if _, ok := parsedJSON["required_skills"].([]any); !ok {
			missingFields = append(missingFields, "required_skills")
		}
		if _, ok := parsedJSON["required_skills_map"].(map[string]any); !ok {
			missingFields = append(missingFields, "required_skills_map")
		}

		if len(missingFields) > 0 {
			s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("StepAnalyze Iteration %d: output missing required fields: %v", i+1, missingFields))
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
				Content: fmt.Sprintf("Your JSON output is missing the following required fields from the execution contract: %s. You MUST include them.", strings.Join(missingFields, ", ")),
			})
			continue
		}

		// Option B+: Validate DAG and Cost Constraints deterministically during planning loop
		analysisDraft := parseAnalysisFinal(parsedJSON)
		if len(analysisDraft.ExecutionUnits) > 0 {
			if errVal := policy.ValidateDAG(analysisDraft.ExecutionUnits, s.maxCost); errVal != nil {
				s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("StepAnalyze Iteration %d: proposed execution plan failed DAG/cost validation: %v", i+1, errVal))
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
					Content: fmt.Sprintf("Your proposed execution plan failed DAG/cost validation checks. Error: %v\n\nPlease adjust the execution_units array by splitting units that exceed the cost limit (limit is %.1f, migration task is cost 5, file creation is cost 2, file modify is cost 1, plus max_risk multiplier). Ensure each unit is small, touches a maximum of 3-4 files, and has no cyclic dependencies. Re-output the corrected JSON specification.", errVal, s.maxCost),
				})
				continue
			}
		}

		return parsedJSON, nil
	}

	return nil, fmt.Errorf("exceeded max iterations (%d)", maxIterations)
}

func (s *AnalyzeStep) writeOpenSpecFiles(ctx context.Context, localPath string, analysis *models.TaskAnalysis) {
	changeName := patch.DeriveChangeName(s.rt.Task)
	changeDir := filepath.Join(localPath, "openspec", "changes", changeName)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to create change directory: %v", err))
		return
	}

	if analysis.ProposalMD == "" {
		analysis.ProposalMD = fmt.Sprintf("## Proposal for %s\n\n%s\n", s.rt.Task.Title, s.rt.Task.Description)
	}
	if analysis.SpecsMD == "" {
		analysis.SpecsMD = fmt.Sprintf("## ADDED Requirements\n\n### Requirement: %s\n%s\n", s.rt.Task.Title, s.rt.Task.Description)
	}
	if analysis.DesignMD == "" {
		analysis.DesignMD = "## Design\n\nImplementation design details.\n"
	}
	if analysis.TasksMD == "" {
		var builder strings.Builder
		builder.WriteString("## Tasks\n\n")
		if len(analysis.ExecutionPhases) > 0 {
			for i, phase := range analysis.ExecutionPhases {
				builder.WriteString(fmt.Sprintf("**%d. %s**\n\n", i+1, phase.Phase))
				for _, task := range phase.Tasks {
					builder.WriteString(fmt.Sprintf("- [ ] %s\n", task))
				}
				builder.WriteString("\n")
			}
		} else {
			builder.WriteString("- [ ] Implement changes\n")
		}
		analysis.TasksMD = builder.String()
	}

	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte(analysis.ProposalMD), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save proposal.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "specs.md"), []byte(analysis.SpecsMD), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save specs.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "design.md"), []byte(analysis.DesignMD), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save design.md: %v", err))
	}
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(analysis.TasksMD), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save tasks.md: %v", err))
	}

	meta := fmt.Sprintf("changeName: %s\ntaskId: %s\nstatus: pending_review\n", changeName, s.rt.Task.ID)
	if err := os.WriteFile(filepath.Join(changeDir, ".openspec.yaml"), []byte(meta), 0o644); err != nil {
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save .openspec.yaml: %v", err))
	}
}

func (s *AnalyzeStep) applyAnalyzePolicy(ctx context.Context, analysis models.TaskAnalysis, fallbackUsed bool) (StepResult, error) {
	oldComplexity := s.rt.Task.Complexity
	analysis.SpecHash = ""
	rawBytes, _ := json.Marshal(analysis)
	hash := sha256.Sum256(rawBytes)
	specHashHex := fmt.Sprintf("%x", hash)
	analysis.SpecHash = specHashHex

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

	affectedFilesStrings := make([]string, len(analysis.AffectedFiles))
	for i, f := range analysis.AffectedFiles {
		affectedFilesStrings[i] = f.File
	}

	specStatus, status := policy.ShouldAutoApproveSpec(
		analysis.Complexity,
		affectedFilesStrings,
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

	if specStatus == models.TaskSpecStatusAutoApproved {
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("OpenSpec auto-approved and frozen. SpecHash: %s", specHashHex))
	} else if specStatus == models.TaskSpecStatusPendingReview {
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("OpenSpec pending human review. SpecHash: %s", specHashHex))
	}

	if specStatus == models.TaskSpecStatusPendingReview || specStatus == models.TaskSpecStatusChangesRequested || specStatus == models.TaskSpecStatusClarificationRequired {
		if specStatus == models.TaskSpecStatusClarificationRequired {
			pauseReason = "workflow paused for human task clarification"
		}
		return nil, workflow.PauseError{Step: workflow.StepAnalyze, Reason: pauseReason}
	}

	if oldComplexity != analysis.Complexity && specStatus == models.TaskSpecStatusAutoApproved {
		return StepResult{"complexity": analysis.Complexity, "spec_status": specStatus}, workflow.ErrGraphChanged
	}

	return StepResult{"complexity": analysis.Complexity, "spec_status": specStatus}, nil
}

// analyzeToolDefinitions returns the registry tool definitions for read/search capabilities.
func (s *AnalyzeStep) analyzeToolDefinitions() []llm.ToolDefinition {
	if s.registry == nil {
		return []llm.ToolDefinition{}
	}
	return s.registry.ToolsForCapabilities([]tool.Capability{tool.CapRead, tool.CapSearch})
}

// executeAnalyzeTool executes a registry tool by name.
func (s *AnalyzeStep) executeAnalyzeTool(ctx context.Context, toolName, arguments string) string {
	var args map[string]any
	if strings.TrimSpace(arguments) != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return fmt.Sprintf("Error: invalid tool arguments JSON: %v", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	if s.registry == nil {
		return "Error: tool registry not configured"
	}

	osPaths := paths.NewOSWorkspacePaths(s.workspaceRoot)
	workspacePath := osPaths.TaskRoot(s.rt.Task.ID).String()

	if s.wkspace != nil {
		if ws, err := s.wkspace.LoadTaskWorkspace(ctx, s.rt.Task); err == nil && ws != nil && len(ws.Repos) > 0 {
			for _, repo := range ws.Repos {
				if s.rt.Task.RepositoryID != nil && repo.RepoID != *s.rt.Task.RepositoryID {
					continue
				}
				if repo.Paths.Main != "" {
					workspacePath = osPaths.RepoMain(s.rt.Task.ID, repo.Name).String()
					break
				}
			}
		}
	}

	call := tool.Call{
		Input:     args,
		Workspace: workspacePath,
		TaskID:    s.rt.Task.ID,
		AgentID:   s.rt.Agent.ID,
		AgentRole: s.rt.Agent.Role,
	}

	res, err := s.registry.Execute(ctx, toolName, call)
	if err != nil {
		return "Error: " + err.Error()
	}

	if !res.Success {
		var errStr string
		if res.Message != "" {
			errStr = res.Message
		} else if len(res.Diagnostics) > 0 {
			errStr = res.Diagnostics[0].Message
		} else {
			errStr = "tool execution failed"
		}
		return "Error: " + errStr
	}

	return res.Output
}

func formatRepoContext(contextOut map[string]any) string {
	var sb strings.Builder

	// 1. Current Branches
	if branches, ok := contextOut["current_branches"].(map[string]any); ok && len(branches) > 0 {
		sb.WriteString("### Current Branches\n")
		for repo, branch := range branches {
			sb.WriteString(fmt.Sprintf("- **%s**: `%v`\n", repo, branch))
		}
		sb.WriteString("\n")
	} else if branches, ok := contextOut["current_branches"].(map[string]string); ok && len(branches) > 0 {
		sb.WriteString("### Current Branches\n")
		for repo, branch := range branches {
			sb.WriteString(fmt.Sprintf("- **%s**: `%s`\n", repo, branch))
		}
		sb.WriteString("\n")
	}

	// 2. Git Logs
	if logs, ok := contextOut["git_logs"].(map[string]any); ok && len(logs) > 0 {
		sb.WriteString("### Git Logs (recent commits)\n")
		for repo, log := range logs {
			sb.WriteString(fmt.Sprintf("- **%s**:\n  ```\n  %v\n  ```\n", repo, log))
		}
		sb.WriteString("\n")
	} else if logs, ok := contextOut["git_logs"].(map[string]string); ok && len(logs) > 0 {
		sb.WriteString("### Git Logs (recent commits)\n")
		for repo, log := range logs {
			sb.WriteString(fmt.Sprintf("- **%s**:\n  ```\n  %s\n  ```\n", repo, log))
		}
		sb.WriteString("\n")
	}

	// 3. Test Commands
	if cmds, ok := contextOut["test_commands"].([]any); ok && len(cmds) > 0 {
		sb.WriteString("### Detected Test Commands\n")
		for _, cmd := range cmds {
			sb.WriteString(fmt.Sprintf("- `%v`\n", cmd))
		}
		sb.WriteString("\n")
	} else if cmds, ok := contextOut["test_commands"].([]string); ok && len(cmds) > 0 {
		sb.WriteString("### Detected Test Commands\n")
		for _, cmd := range cmds {
			sb.WriteString(fmt.Sprintf("- `%s`\n", cmd))
		}
		sb.WriteString("\n")
	}

	// 4. CI Configs
	if configs, ok := contextOut["ci_configs"].([]any); ok && len(configs) > 0 {
		sb.WriteString("### CI Configurations\n")
		for _, config := range configs {
			sb.WriteString(fmt.Sprintf("- `%v`\n", config))
		}
		sb.WriteString("\n")
	} else if configs, ok := contextOut["ci_configs"].([]string); ok && len(configs) > 0 {
		sb.WriteString("### CI Configurations\n")
		for _, config := range configs {
			sb.WriteString(fmt.Sprintf("- `%s`\n", config))
		}
		sb.WriteString("\n")
	}

	// 5. Conventions
	if convs, ok := contextOut["conventions"].(map[string]any); ok && len(convs) > 0 {
		sb.WriteString("### Repository Conventions\n")
		for file, content := range convs {
			sb.WriteString(fmt.Sprintf("#### File: `%s`\n```\n%v\n```\n", file, content))
		}
		sb.WriteString("\n")
	} else if convs, ok := contextOut["conventions"].(map[string]string); ok && len(convs) > 0 {
		sb.WriteString("### Repository Conventions\n")
		for file, content := range convs {
			sb.WriteString(fmt.Sprintf("#### File: `%s`\n```\n%s\n```\n", file, content))
		}
		sb.WriteString("\n")
	}

	// 6. Architectures
	if archs, ok := contextOut["architectures"].(map[string]any); ok && len(archs) > 0 {
		sb.WriteString("### Architectural Guidelines\n")
		for repo, content := range archs {
			sb.WriteString(fmt.Sprintf("#### Repository: **%s**\n%v\n", repo, content))
		}
		sb.WriteString("\n")
	} else if archs, ok := contextOut["architectures"].(map[string]string); ok && len(archs) > 0 {
		sb.WriteString("### Architectural Guidelines\n")
		for repo, content := range archs {
			sb.WriteString(fmt.Sprintf("#### Repository: **%s**\n%s\n", repo, content))
		}
		sb.WriteString("\n")
	}

	// 7. Contributings
	if contribs, ok := contextOut["contributings"].(map[string]any); ok && len(contribs) > 0 {
		sb.WriteString("### Contributing Guidelines\n")
		for repo, content := range contribs {
			sb.WriteString(fmt.Sprintf("#### Repository: **%s**\n%v\n", repo, content))
		}
		sb.WriteString("\n")
	} else if contribs, ok := contextOut["contributings"].(map[string]string); ok && len(contribs) > 0 {
		sb.WriteString("### Contributing Guidelines\n")
		for repo, content := range contribs {
			sb.WriteString(fmt.Sprintf("#### Repository: **%s**\n%s\n", repo, content))
		}
		sb.WriteString("\n")
	}

	// 8. Context Cache directory tree (if any)
	if cacheJSON, ok := contextOut["context_cache"].(string); ok && cacheJSON != "" {
		var cache struct {
			DirectoryTree    string `json:"directory_tree"`
			RepoMap          string `json:"repo_map"`
			SemanticSnippets []struct {
				Path      string  `json:"path"`
				StartLine int     `json:"start_line"`
				EndLine   int     `json:"end_line"`
				Content   string  `json:"content"`
				Relevance float64 `json:"relevance"`
				Retriever string  `json:"retriever"`
			} `json:"semantic_snippets"`
		}
		if err := json.Unmarshal([]byte(cacheJSON), &cache); err == nil {
			if cache.RepoMap != "" {
				sb.WriteString("### Repository Map\n```\n")
				sb.WriteString(cache.RepoMap)
				sb.WriteString("\n```\n\n")
			}
			if len(cache.SemanticSnippets) > 0 {
				sb.WriteString("### Relevant Code Snippets\n")
				for i, sn := range cache.SemanticSnippets {
					sb.WriteString(fmt.Sprintf("#### Snippet %d: `%s:%d-%d` (relevance: %.2f, source: %s)\n```\n%s\n```\n\n", i+1, sn.Path, sn.StartLine, sn.EndLine, sn.Relevance, sn.Retriever, sn.Content))
				}
			}
		}
	}

	return strings.TrimSpace(sb.String())
}
