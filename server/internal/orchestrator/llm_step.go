package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// stepIsAgentic reports whether stepID should drive a native tool-calling loop instead of the
// single-shot Chat() path (Issue 1+2). Coding steps are only agentic once their edit tools are
// routed through the boundary-checked executor (see NewBoundaryCheckedToolExecutor).
func stepIsAgentic(stepID string) bool {
	return stepID == workflow.StepReview ||
		strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend) ||
		stepID == workflow.StepFix
}

// agenticWorkspaceRole returns the worktree role a step's tool calls should operate in ("" for
// the main checkout).
func agenticWorkspaceRole(stepID string) string {
	if strings.HasPrefix(stepID, workflow.StepCodeBackend) {
		return "backend"
	}
	if strings.HasPrefix(stepID, workflow.StepCodeFrontend) {
		return "frontend"
	}
	return ""
}

func (o *Orchestrator) runLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	o.initCheckpoints()
	agentRole := ""
	if agent != nil {
		agentRole = agent.Role
	}
	resolvedRole := tool.EffectiveRoleForStep(stepID, agentRole, task)

	var tools []llm.ToolDefinition
	if o.capManager != nil && agent != nil {
		tools = o.capManager.ToolsForRole(resolvedRole)
	}
	var assemble llmrunner.PromptAssembler
	if o.prompts != nil {
		var budgetTrace *prompts.BudgetTrace
		ctx, budgetTrace = prompts.WithBudgetTrace(ctx)
		_ = budgetTrace
		assemble = func(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, error) {
			messages, _, err := o.prompts.AssembleForAgent(ctx, task, agent, history, tools)
			return messages, err
		}
	}

	runner := llmrunner.Runner{
		WorkspaceRoot:           o.workspaceRoot,
		Provider:                o.llm,
		AssemblePrompt:          assemble,
		Projects:                o.projects,
		ReadAffectedFileContent: o.readAffectedFileContent,
		SaveArtifact:            o.checkpoints.SaveArtifact,
		WriteTrace:              o.writeLLMCallTrace,
		Log:                     o.log,
		MaxToolResultChars:      o.maxToolResultChars,
		CaptureDiff: func(ctx context.Context, task *models.Task, agent *models.Agent, worktreeSuffix string) (string, error) {
			o.initRepoutil()
			return o.repoutil.CaptureWorkspaceDiff(ctx, task, agent, stepID, worktreeSuffix)
		},
		// o.llm is a router (Gateway/NineRouter) that picks the underlying model per-call from
		// RouteOptions, so the serving provider isn't known until the call happens — "default"
		// (plain-text sections) is the only rendering choice knowable up front. The "anthropic"
		// tag-wrapped variant stays available on DefaultPromptCompiler for a future caller that
		// does have that info (e.g. a fixed single-provider deployment).
		Compiler: prompts.NewDefaultPromptCompiler("default"),
	}
	if stepIsAgentic(stepID) && o.registry != nil && len(tools) > 0 {
		agentID := ""
		if agent != nil {
			agentID = agent.ID
		}
		workspace := o.resolveAgenticWorkspace(ctx, task, agenticWorkspaceRole(stepID))
		runner.Tools = tools
		runner.ToolExecutor = steps.NewBoundaryCheckedToolExecutor(o.registry, workspace, task, agentID, resolvedRole, o.tasks)
	}

	if models.IsStateMachineEnabled(ctx) && o.checkpoints != nil && assemble != nil {
		if snap, exists := o.checkpoints.GetLatestExecutionSnapshot(ctx, task.ID, stepID); exists {
			// Reconstruct the exact same initial prompt Run() would build, so the hash is
			// comparable to the one saveExecutionSnapshot computed (see BuildInitialMessages).
			messages, err := runner.BuildInitialMessages(ctx, task, agent, stepID, instruction)
			if err == nil {
				rawMsgs, _ := json.Marshal(messages)
				h := sha256.Sum256(rawMsgs)
				currentPromptHash := hex.EncodeToString(h[:])
				var analysis models.TaskAnalysis
				if len(task.Analysis) > 0 {
					_ = json.Unmarshal(task.Analysis, &analysis)
				}
				var ir models.ExecutionIR
				role := agenticWorkspaceRole(stepID)
				for _, node := range analysis.ExecutionIRs {
					for _, unit := range analysis.ExecutionUnits {
						if unit.ID == node.NodeID {
							if strings.ToLower(unit.ExecutionProfile.Agent) == role {
								ir = node
								break
							}
						}
					}
				}
				if ir.NodeID == "" && len(analysis.ExecutionIRs) == 1 {
					ir = analysis.ExecutionIRs[0]
				}
				if ir.NodeID == "" {
					ir = models.ExecutionIR{
						SchemaVersion: models.CurrentExecutionIRSchemaVersion,
						NodeID:        "default",
						Intent:        models.Intent{Capability: stepID, Operation: "modify"},
						Constraints:   []string{},
						Acceptance:    []string{},
					}
				}
				resolvedTargets := analysis.ExecutionIRTargets[ir.NodeID]
				currentSemanticHash := models.ComputeSemanticHash(ir, resolvedTargets)

				matched := false
				if currentPromptHash == snap.PromptHash {
					o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: PromptHash verified. Resuming with byte-identical execution state.", stepID))
					matched = true
				} else if snap.SemanticHash != "" && snap.SemanticHash == currentSemanticHash {
					o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: SemanticHash verified (%s). Prompt wording changed, skipping re-reasoning.", stepID, snap.SemanticHash))
					matched = true
				}

				if matched {
					// Apply snapshot diff to restore workspace
					o.initRepoutil()
					worktreeSuffix := ""
					if task.Complexity != models.TaskComplexityEasy {
						if strings.HasPrefix(stepID, workflow.StepCodeBackend) {
							worktreeSuffix = "-be-worktree"
						} else if strings.HasPrefix(stepID, workflow.StepCodeFrontend) {
							worktreeSuffix = "-fe-worktree"
						}
					}

					if errReset := o.repoutil.ResetRoleWorktrees(ctx, task, agent, worktreeSuffix); errReset != nil {
						o.log(ctx, task.ID, nil, "error", fmt.Sprintf("failed to reset worktree for resume: %v", errReset))
					} else if snap.WorkspaceDiff != "" {
						if errApply := o.repoutil.ApplyPatch(ctx, task, agent, stepID+"_restore", snap.WorkspaceDiff, worktreeSuffix); errApply != nil {
							o.log(ctx, task.ID, nil, "error", fmt.Sprintf("failed to restore snapshot diff for resume: %v", errApply))
						}
					}

					var latestResponse map[string]any
					if arts, err := o.checkpoints.Artifacts.ListByTaskID(ctx, task.ID); err == nil {
						for i := len(arts) - 1; i >= 0; i-- {
							art := arts[i]
							if (art.Step == stepID || strings.HasPrefix(art.Step, stepID+"_cycle_")) && art.Type == "llm_response" {
								_ = json.Unmarshal(art.Payload, &latestResponse)
								break
							}
						}
					}
					if latestResponse != nil {
						return latestResponse, nil
					}
				} else {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("step %s: PromptHash mismatch (current: %s, stored: %s). Replaying execution state instead of skipping.", stepID, currentPromptHash, snap.PromptHash))
				}
			}
		}
	}
	return runner.Run(ctx, task, agent, jobID, stepID, instruction)
}

// resolveAgenticWorkspace returns the physical repo checkout that tool calls (in the agentic
// loop) should operate on, so relative paths supplied by the LLM (e.g. "internal/x.go") resolve
// the same way they do for the rest of the pipeline's repo-root conventions. role selects a
// role-specific worktree (matching setupSandbox's "-be-worktree"/"-fe-worktree" convention) or
// the main checkout when empty. Falls back to the flat task workspace root if no repo metadata
// is available.
func (o *Orchestrator) resolveAgenticWorkspace(ctx context.Context, task *models.Task, role string) string {
	o.initWkspace()
	ws, err := o.wkspace.LoadTaskWorkspace(ctx, task)
	if err != nil || ws == nil || len(ws.Repos) == 0 {
		return sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	}
	wp := paths.NewOSWorkspacePaths(filepath.Dir(ws.Root))
	isEasy := task.Complexity == models.TaskComplexityEasy
	if len(ws.Repos) != 1 {
		return wp.CodeRoot(task.ID).String()
	}
	repoName := ws.Repos[0].Name
	if role == "" || isEasy {
		return wp.RepoMain(task.ID, repoName).String()
	}
	return wp.RepoWorktreeDir(task.ID, repoName, role).String()
}

// stepRequiresEditCaps and effectiveRoleForStep moved to internal/tool (tool.StepRequiresEditCaps,
// tool.EffectiveRoleForStep) so orchestrator and prompts share one implementation instead of two
// copies that could silently diverge — see internal/tool/rolepolicy.go.
