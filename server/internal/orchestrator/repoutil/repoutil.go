package repoutil

import (
	"context"
	"encoding/json"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type Manager struct {
	WorkspaceRoot              string
	ListRepositories           func(ctx context.Context, projectID string) ([]models.Repository, error)
	GetTaskWorkspace           func(task *models.Task) *models.TaskWorkspace
	LoadTaskWorkspace          func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error)
	SaveTaskWorkspaceMetadata  func(task *models.Task, ws *models.TaskWorkspace) error
	FindRepoWorkspaceByPath    func(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error)
	ContainerPathForHostPath   func(task *models.Task, hostPath string, worktreeSuffix string) string
	RunSandboxStep             func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, script string) (map[string]any, error)
	RunSandboxStepInWorktree   func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, script string, suffix string) (map[string]any, error)
	SandboxGitGetChangedFiles  func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
	SandboxGitGetDiff          func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error)
	SandboxGitGetWorkspaceDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, worktreeSuffix string) (string, error)
	SandboxGitGetPRDiff        func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error)
	Log                        func(ctx context.Context, taskID string, jobID *string, level string, message string)
	UpdateTaskAnalysis         func(ctx context.Context, taskID string, analysis json.RawMessage) error
	DefaultAgentName           string
	DefaultAgentEmail          string
}

func NewManager(
	workspaceRoot string,
	listRepositories func(ctx context.Context, projectID string) ([]models.Repository, error),
	getTaskWorkspace func(task *models.Task) *models.TaskWorkspace,
	loadTaskWorkspace func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error),
	saveTaskWorkspaceMetadata func(task *models.Task, ws *models.TaskWorkspace) error,
	findRepoWorkspaceByPath func(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error),
	containerPathForHostPath func(task *models.Task, hostPath string, worktreeSuffix string) string,
	runSandboxStep func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, script string) (map[string]any, error),
	runSandboxStepInWorktree func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, script string, suffix string) (map[string]any, error),
	sandboxGitGetChangedFiles func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error),
	sandboxGitGetDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error),
	sandboxGitGetWorkspaceDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, worktreeSuffix string) (string, error),
	sandboxGitGetPRDiff func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error),
	log func(ctx context.Context, taskID string, jobID *string, level string, message string),
	updateTaskAnalysis func(ctx context.Context, taskID string, analysis json.RawMessage) error,
	defaultAgentName string,
	defaultAgentEmail string,
) *Manager {
	return &Manager{
		WorkspaceRoot:              workspaceRoot,
		ListRepositories:           listRepositories,
		GetTaskWorkspace:           getTaskWorkspace,
		LoadTaskWorkspace:          loadTaskWorkspace,
		SaveTaskWorkspaceMetadata:  saveTaskWorkspaceMetadata,
		FindRepoWorkspaceByPath:    findRepoWorkspaceByPath,
		ContainerPathForHostPath:   containerPathForHostPath,
		RunSandboxStep:             runSandboxStep,
		RunSandboxStepInWorktree:   runSandboxStepInWorktree,
		SandboxGitGetChangedFiles:  sandboxGitGetChangedFiles,
		SandboxGitGetDiff:          sandboxGitGetDiff,
		SandboxGitGetWorkspaceDiff: sandboxGitGetWorkspaceDiff,
		SandboxGitGetPRDiff:        sandboxGitGetPRDiff,
		Log:                        log,
		UpdateTaskAnalysis:         updateTaskAnalysis,
		DefaultAgentName:           defaultAgentName,
		DefaultAgentEmail:          defaultAgentEmail,
	}
}
