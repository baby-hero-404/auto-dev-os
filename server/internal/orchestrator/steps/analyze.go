package steps

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
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
	maxCost       float64
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
	}
}

func (s *AnalyzeStep) ID() string                         { return workflow.StepAnalyze }
func (s *AnalyzeStep) StatusOnResume(_ StepResult) string { return models.TaskStatusAnalyzing }

func (s *AnalyzeStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	localPath := sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID)
	ctx = context.WithValue(ctx, provider.WorkspaceRootKey, localPath)

	if s.prompts != nil {
		messages, tools, err := s.prompts.AssembleForAgent(ctx, *s.rt.Task, s.rt.Agent, nil)
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

	instruction := buildAnalyzeInstruction(stepCtx)
	messages, err := s.buildAnalyzeMessages(ctx, instruction)
	if err != nil {
		return models.TaskAnalysis{}, false, err
	}

	parsedFinal, loopErr := s.runAnalyzeLLMLoop(ctx, messages)
	if loopErr != nil || parsedFinal == nil {
		if loopErr == nil {
			loopErr = fmt.Errorf("LLM returned empty or nil spec JSON")
		}
		s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("agent failed to output a valid final spec JSON after retry iterations: %v. Falling back to human review spec.", loopErr))
		return deriveWorkflowAnalysis(s.rt.Task), true, nil
	}

	return parseAnalysisFinal(parsedFinal), false, nil
}

func (s *AnalyzeStep) buildAnalyzeMessages(ctx context.Context, instruction string) ([]llm.Message, error) {
	var messages []llm.Message
	var err error
	if s.prompts != nil {
		stepCtx := context.WithValue(ctx, prompts.StepIDCtxKey, workflow.StepAnalyze)
		messages, _, err = s.prompts.AssembleForAgent(stepCtx, *s.rt.Task, s.rt.Agent, nil)
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
	instruction := "Analyze this task and output the proposed specification as a valid JSON object matching the schema and template requested in the system instructions."
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
