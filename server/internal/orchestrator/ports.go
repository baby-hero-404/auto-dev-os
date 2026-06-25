package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type WorkspaceResolver interface {
	getTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error)
	hostWorktreePath(task *models.Task, repoPath string, worktreeSuffix string) string
	containerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string
}

type PatchApplier interface {
	applyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error
}

type DiffProvider interface {
	captureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error)
	capturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error)
}

type TestRunner interface {
	runTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error)
}

type LLMStepRunner interface {
	runLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error)
}

var (
	_ WorkspaceResolver = (*Orchestrator)(nil)
	_ PatchApplier      = (*Orchestrator)(nil)
	_ DiffProvider      = (*Orchestrator)(nil)
	_ TestRunner        = (*Orchestrator)(nil)
	_ LLMStepRunner     = (*Orchestrator)(nil)
)
