package steps

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/checkpoint"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/repoutil"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
}

type WorkflowRepository interface {
	UpdateJob(ctx context.Context, jobID string, updates map[string]any) (*models.WorkflowJob, error)
	DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
	CreateCheckpoint(ctx context.Context, checkpoint models.WorkflowCheckpoint) error
}

type ProjectRepository interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

type RepositoryRepository interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error)
}

type AgentAssigner interface {
	Assign(ctx context.Context, task *models.Task) (*models.Agent, error)
	AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error)
	MarkRunning(ctx context.Context, agentID string) error
	Release(ctx context.Context, agentID string) error
}

type GitOpsClient interface {
	CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error
	CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
}

type ArtifactRepository interface {
	Create(ctx context.Context, artifact *models.WorkflowArtifact) error
	ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error)
	ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error)
}

type PromptBuilder interface {
	Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error)
	AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error)
}

type WorkspaceLoader interface {
	LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error)
	SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error
}

type Deps struct {
	Tasks         TaskRepository
	Workflows     WorkflowRepository
	Projects      ProjectRepository
	Repos         RepositoryRepository
	Agents        AgentAssigner
	LLM           llm.Provider
	Prompts       PromptBuilder
	Runtime       sandbox.Runtime
	Wkspace       WorkspaceLoader
	Checkpoints   *checkpoint.Store
	RepoUtil      *repoutil.Manager
	SandboxGit    gitops.SandboxGitClient
	GitOps        GitOpsClient
	Artifacts     ArtifactRepository
	WorkspaceRoot string

	// Function delegates
	RunLLMStep               func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (map[string]any, error)
	RunSandboxStep           func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error)
	RunSandboxStepInWorktree func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string, suffix string) (map[string]any, error)
	RunTargetedTests         func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error)
	SaveArtifact             func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error
	UpdateTaskStatus         func(ctx context.Context, taskID string, newStatus string) (*models.Task, error)
	Log                      func(ctx context.Context, taskID string, jobID *string, level string, message string)
	ContainerPathForHostPath func(task *models.Task, hostPath string, worktreeSuffix string) string
	ReadAffectedFileContent  func(ctx context.Context, task *models.Task, filePath string) (string, bool)
	WriteLLMCallTrace        func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed map[string]any)
}
