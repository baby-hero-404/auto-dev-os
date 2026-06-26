package orchestrator

import (
	"context"

	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) runTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error) {
	o.initRepoutil()
	o.initCheckpoints()
	runner := orchtester.Runner{
		ResolveRepoHostPath:      o.repoutil.GetTaskRepoHostPath,
		HostWorktreePath:         o.repoutil.HostWorktreePath,
		ContainerPathForHostPath: o.containerPathForHostPath,
		RunSandboxStepInWorktree: o.runSandboxStepInWorktree,
		SaveArtifact:             o.checkpoints.SaveArtifact,
		Log:                      o.log,
	}
	return runner.RunTargetedTests(ctx, task, agent, jobID, stepName, changedFiles, worktreeSuffix)
}
