package wkspace

import (
	"context"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type WorkspaceRetention struct {
	Retention time.Duration
	Interval  time.Duration
}

type Manager struct {
	Tasks         TaskRepository
	Workflows     WorkflowRepository
	Repositories  RepositoryRepository
	GitOps        GitOpsClient
	Artifacts     ArtifactRepository
	WorkspaceRoot string
	Retention     WorkspaceRetention
	LockCancels   sync.Map
	LockConns     sync.Map

	Log        func(ctx context.Context, taskID string, jobID *string, level string, message string)
	ApplyPatch func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, patchText string, worktreeSuffix string) error
}

func NewManager(
	tasks TaskRepository,
	workflows WorkflowRepository,
	repositories RepositoryRepository,
	gitOps GitOpsClient,
	artifacts ArtifactRepository,
	workspaceRoot string,
	retention WorkspaceRetention,
	log func(ctx context.Context, taskID string, jobID *string, level string, message string),
	applyPatch func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, patchText string, worktreeSuffix string) error,
) *Manager {
	return &Manager{
		Tasks:         tasks,
		Workflows:     workflows,
		Repositories:  repositories,
		GitOps:        gitOps,
		Artifacts:     artifacts,
		WorkspaceRoot: workspaceRoot,
		Retention:     retention,
		Log:           log,
		ApplyPatch:    applyPatch,
	}
}
