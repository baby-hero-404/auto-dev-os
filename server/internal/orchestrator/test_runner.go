package orchestrator

import (
	"context"

	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) runTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error) {
	runner := orchtester.Runner{
		ResolveRepoHostPath:      o.getTaskRepoHostPath,
		HostWorktreePath:         o.hostWorktreePath,
		ContainerPathForHostPath: o.containerPathForHostPath,
		RunSandboxStepInWorktree: o.runSandboxStepInWorktree,
		SaveArtifact:             o.saveArtifact,
		Log:                      o.log,
	}
	return runner.RunTargetedTests(ctx, task, agent, jobID, stepName, changedFiles, worktreeSuffix)
}
