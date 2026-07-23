package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/checkpoint"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/repoutil"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// safeUpdateTaskAnalysis routes analysis mutations from the patch/repoutil
// layer through the same locked read-modify-write helper the step layer
// uses, so the two can never race and overwrite each other's writes.
func (o *Orchestrator) safeUpdateTaskAnalysis(ctx context.Context, task *models.Task, mutate func(*models.TaskAnalysis) bool) error {
	return steps.UpdateTaskAnalysis(ctx, task.ID, o.tasks, task, mutate)
}

func (o *Orchestrator) initWkspace() {
	if o.wkspace == nil {
		o.wkspace = wkspace.NewManager(
			o.tasks,
			o.workflows,
			o.repositories,
			o.gitOps,
			o.artifacts,
			o.workspaceRoot,
			wkspace.WorkspaceRetention{
				Retention: o.retention.Retention,
				Interval:  o.retention.Interval,
			},
			o.log,
			func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, patchText string, worktreeSuffix string) error {
				o.initRepoutil()
				return o.repoutil.ApplyPatch(ctx, task, agent, stepName, patchText, worktreeSuffix)
			},
		)
	} else {
		o.wkspace.Tasks = o.tasks
		o.wkspace.Workflows = o.workflows
		o.wkspace.Repositories = o.repositories
		o.wkspace.GitOps = o.gitOps
		o.wkspace.Artifacts = o.artifacts
		o.wkspace.WorkspaceRoot = o.workspaceRoot
		o.wkspace.Retention = wkspace.WorkspaceRetention{
			Retention: o.retention.Retention,
			Interval:  o.retention.Interval,
		}
	}
}

func (o *Orchestrator) StartWorkspacePruner(ctx context.Context) {
	o.initWkspace()
	o.wkspace.StartWorkspacePruner(ctx)
}

func (o *Orchestrator) StartLogPruner(ctx context.Context, retentionDays int, fileRoot string) {
	o.initWkspace()
	o.wkspace.LogFileRoot = fileRoot
	o.wkspace.StartLogPruner(ctx, retentionDays, fileRoot)
}

func (o *Orchestrator) ensureWorkspaceCloned(ctx context.Context, task *models.Task, agent *models.Agent, jobID string) error {
	o.initWkspace()
	return o.wkspace.EnsureWorkspaceCloned(ctx, task, agent, jobID)
}

func (o *Orchestrator) cleanupWorkspaceAfterFinalState(ctx context.Context, taskID string) {
	o.initWkspace()
	o.wkspace.CleanupWorkspaceAfterFinalState(ctx, taskID)
}

func (o *Orchestrator) releaseWorkspaceLock(taskID string) {
	o.initWkspace()
	o.wkspace.ReleaseWorkspaceLock(taskID)
}

func (o *Orchestrator) RemoveWorkspace(taskID string) error {
	o.initWkspace()
	return o.wkspace.RemoveWorkspace(taskID)
}

func (o *Orchestrator) initCheckpoints() {
	if o.checkpoints == nil {
		o.checkpoints = checkpoint.NewStore(
			o.workflows,
			o.artifacts,
			o.log,
		)
	} else {
		o.checkpoints.Workflows = o.workflows
		o.checkpoints.Artifacts = o.artifacts
	}
}

func (o *Orchestrator) initRepoutil() {
	o.initWkspace()
	if o.repoutil == nil {
		var listRepos func(ctx context.Context, projectID string) ([]models.Repository, error)
		if o.repositories != nil {
			listRepos = o.repositories.ListByProjectID
		}
		var getChangedFiles func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
		var getDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error)
		var getWorkspaceDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, worktreeSuffix string) (string, error)
		var getPRDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error)
		if o.sandboxGit != nil {
			getChangedFiles = o.sandboxGit.GetChangedFiles
			getDiff = o.sandboxGit.GetDiff
			getWorkspaceDiff = o.sandboxGit.GetWorkspaceDiff
			getPRDiff = o.sandboxGit.GetPRDiff
		}
		o.repoutil = repoutil.NewManager(
			o.workspaceRoot,
			listRepos,
			o.wkspace.GetTaskWorkspace,
			o.wkspace.LoadTaskWorkspace,
			o.wkspace.SaveTaskWorkspaceMetadata,
			o.wkspace.FindRepoWorkspaceByPath,
			o.containerPathForHostPath,
			o.runSandboxStep,
			o.runSandboxStepInWorktree,
			getChangedFiles,
			getDiff,
			getWorkspaceDiff,
			getPRDiff,
			o.log,
			o.safeUpdateTaskAnalysis,
			o.gitConfig.DefaultAgentName,
			o.gitConfig.DefaultAgentEmail,
		)
	} else {
		o.repoutil.WorkspaceRoot = o.workspaceRoot
		o.repoutil.DefaultAgentName = o.gitConfig.DefaultAgentName
		o.repoutil.DefaultAgentEmail = o.gitConfig.DefaultAgentEmail
		o.repoutil.UpdateTaskAnalysis = o.safeUpdateTaskAnalysis
		if o.repositories != nil {
			o.repoutil.ListRepositories = o.repositories.ListByProjectID
		}
		if o.sandboxGit != nil {
			o.repoutil.SandboxGitGetChangedFiles = o.sandboxGit.GetChangedFiles
			o.repoutil.SandboxGitGetDiff = o.sandboxGit.GetDiff
			o.repoutil.SandboxGitGetWorkspaceDiff = o.sandboxGit.GetWorkspaceDiff
			o.repoutil.SandboxGitGetPRDiff = o.sandboxGit.GetPRDiff
		}
	}
}
