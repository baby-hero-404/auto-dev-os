package orchestrator

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
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
	var tools []llm.ToolDefinition
	if o.capManager != nil && agent != nil {
		tools = o.capManager.ToolsForRole(agent.Role)
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
	}
	if stepIsAgentic(stepID) && o.registry != nil && len(tools) > 0 {
		agentID, agentRole := "", ""
		if agent != nil {
			agentID, agentRole = agent.ID, agent.Role
		}
		workspace := o.resolveAgenticWorkspace(ctx, task, agenticWorkspaceRole(stepID))
		runner.Tools = tools
		runner.ToolExecutor = steps.NewBoundaryCheckedToolExecutor(o.registry, workspace, task, agentID, agentRole, o.tasks)
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
