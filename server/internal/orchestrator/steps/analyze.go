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
		s.log.Log(ctx, s.rt.Task.ID, nil, "error", fmt.Sprintf("agent failed to output a final spec JSON: %v", loopErr))
		return models.TaskAnalysis{}, false, fmt.Errorf("analyze step failed: %w", loopErr)
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

		// Contract Validation Chặt Chẽ
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
				analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{File: s})
			} else if m, ok := item.(map[string]any); ok {
				repo, _ := m["repo"].(string)
				file, _ := m["file"].(string)
				conf, _ := m["confidence"].(float64)
				reason, _ := m["reason"].(string)
				analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{
					Repo:       repo,
					File:       file,
					Confidence: conf,
					Reason:     reason,
				})
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
	if phases, ok := parsedFinal["execution_phases"].([]any); ok {
		for _, phaseItem := range phases {
			if pMap, ok := phaseItem.(map[string]any); ok {
				phaseName, _ := pMap["phase"].(string)
				var tasks []string
				if tArr, ok := pMap["tasks"].([]any); ok {
					for _, t := range tArr {
						if ts, ok := t.(string); ok {
							tasks = append(tasks, ts)
						}
					}
				}
				analysis.ExecutionPhases = append(analysis.ExecutionPhases, models.ExecutionPhase{
					Phase: phaseName,
					Tasks: tasks,
				})
			}
		}
	}
	if units, ok := parsedFinal["execution_units"].([]any); ok {
		for _, unitItem := range units {
			if uMap, ok := unitItem.(map[string]any); ok {
				var unit models.ExecutionUnit
				unit.ID, _ = uMap["id"].(string)
				unit.Objective, _ = uMap["objective"].(string)
				if tArr, ok := uMap["tasks"].([]any); ok {
					for _, t := range tArr {
						if ts, ok := t.(string); ok {
							unit.Tasks = append(unit.Tasks, ts)
						}
					}
				}
				if prof, ok := uMap["execution_profile"].(map[string]any); ok {
					unit.ExecutionProfile.Agent, _ = prof["agent"].(string)
					if sks, ok := prof["skills"].([]any); ok {
						for _, sk := range sks {
							if sksStr, ok := sk.(string); ok {
								unit.ExecutionProfile.Skills = append(unit.ExecutionProfile.Skills, sksStr)
							}
						}
					}
				}
				if cons, ok := uMap["constraints"].(map[string]any); ok {
					unit.Constraints.Parallelizable, _ = cons["parallelizable"].(bool)
					if mf, ok := cons["max_files"].(float64); ok {
						unit.Constraints.MaxFiles = int(mf)
					}
					if et, ok := cons["estimated_tokens"].(float64); ok {
						unit.Constraints.EstimatedTokens = int(et)
					}
					unit.Constraints.MaxRisk, _ = cons["max_risk"].(string)
				}
				if deps, ok := uMap["dependencies"].([]any); ok {
					for _, dep := range deps {
						if depStr, ok := dep.(string); ok {
							unit.Dependencies = append(unit.Dependencies, depStr)
						}
					}
				}
				analysis.ExecutionUnits = append(analysis.ExecutionUnits, unit)
			}
		}
	}
	// Runtime Adapter: Map ExecutionUnits to ExecutionPhases for old UI compatibility
	if len(analysis.ExecutionPhases) == 0 && len(analysis.ExecutionUnits) > 0 {
		for _, unit := range analysis.ExecutionUnits {
			analysis.ExecutionPhases = append(analysis.ExecutionPhases, models.ExecutionPhase{
				Phase: fmt.Sprintf("%s (%s)", unit.Objective, unit.ExecutionProfile.Agent),
				Tasks: unit.Tasks,
			})
		}
	}
	if questions, ok := parsedFinal["clarification_questions"].([]any); ok {
		for _, item := range questions {
			if s, ok := item.(string); ok {
				analysis.ClarificationQuestions = append(analysis.ClarificationQuestions, s)
			}
		}
	}
	if boundaries, ok := parsedFinal["execution_boundaries"]; ok {
		if arr, ok := boundaries.([]any); ok {
			for _, b := range arr {
				if bmap, ok := b.(map[string]any); ok {
					var boundary models.ExecutionBoundary
					if m, ok := bmap["module"].(string); ok {
						boundary.Module = m
					}
					if r, ok := bmap["root"].(string); ok {
						boundary.Root = r
					}
					if rn, ok := bmap["repo_name"].(string); ok {
						boundary.RepoName = rn
					}
					if rid, ok := bmap["repository_id"].(string); ok {
						boundary.RepositoryID = rid
					}
					if caps, ok := bmap["capabilities"].([]any); ok {
						for _, cp := range caps {
							if cps, ok := cp.(string); ok {
								boundary.Capabilities = append(boundary.Capabilities, cps)
							}
						}
					}
					analysis.ExecutionBoundaries = append(analysis.ExecutionBoundaries, boundary)
				}
			}
		} else if bmap, ok := boundaries.(map[string]any); ok {
			// Backward compatibility: map[string][]string
			for k, v := range bmap {
				var boundary models.ExecutionBoundary
				boundary.Module = k
				if arr, ok := v.([]any); ok && len(arr) > 0 {
					if firstPath, ok := arr[0].(string); ok {
						boundary.Root = filepath.Dir(firstPath) + "/"
					}
					for _, p := range arr {
						if _, ok := p.(string); ok {
							boundary.Capabilities = []string{"modify_existing", "create_test"}
						}
					}
				}
				analysis.ExecutionBoundaries = append(analysis.ExecutionBoundaries, boundary)
			}
		}
	}
	if criteria, ok := parsedFinal["acceptance_criteria"].([]any); ok {
		for _, c := range criteria {
			if cmap, ok := c.(map[string]any); ok {
				analysis.AcceptanceCriteria = append(analysis.AcceptanceCriteria, cmap)
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
		AffectedFiles: []models.AffectedFile{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPhases: []models.ExecutionPhase{
			{
				Phase: "Automated Execution",
				Tasks: []string{
					"Assemble prompt with role, rules, and retrieved context.",
					"Decompose work into typed subtasks.",
					"Run backend and frontend coding tracks in parallel sandboxes.",
					"Merge, review, fix, test, and prepare PR approval checkpoint.",
				},
			},
		},
		ClarificationQuestions: questions,
	}
}
