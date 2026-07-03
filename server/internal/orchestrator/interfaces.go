package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// AgentAssigner manages agent lifecycle for task execution.
type AgentAssigner interface {
	Assign(ctx context.Context, task *models.Task) (*models.Agent, error)
	AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error)
	MarkRunning(ctx context.Context, agentID string) error
	Release(ctx context.Context, agentID string) error
}

// PromptBuilder assembles LLM messages and tool definitions for agents.
type PromptBuilder interface {
	Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error)
	AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error)
}

// GitOpsClient handles git operations (clone, branch, push, PR).
type GitOpsClient interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	// CloneForTask clones a repository resolving credentials from the linked GitAccount.
	// Use this instead of CloneRepo inside the orchestrator so the git account token
	// is never stored in the task/repo model and is always resolved at clone time.
	CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error)
	CreateBranch(ctx context.Context, localPath, repoURL, branchName string) error
	CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error
	CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
	MergePullRequest(ctx context.Context, repoURL, prURL string) error
}

// MemoryRecorder tracks episodic memories across workflow sessions.
type MemoryRecorder interface {
	SessionStart(ctx context.Context, agentID string, task *models.Task) ([]models.EpisodicMemory, error)
	PostStepRecord(ctx context.Context, agentID string, task *models.Task, sessionID, stepID, status string, output map[string]any)
	SessionEnd(ctx context.Context, agentID string, task *models.Task, sessionID, finalStatus string)
}

// LearningRecorder evaluates task outcomes and suggests improvements.
type LearningRecorder interface {
	ComputeConfidence(ctx context.Context, agentID, complexity string) float64
	EvaluateOutcome(ctx context.Context, task *models.Task, job *models.WorkflowJob)
	DetectPatterns(ctx context.Context, agentID string)
	SuggestRuleFromErrors(ctx context.Context, agentID string)
	SuggestPromptPatch(ctx context.Context, task *models.Task, job *models.WorkflowJob)
}

// ArtifactRepository persists workflow step artifacts.
type ArtifactRepository interface {
	Create(ctx context.Context, artifact *models.WorkflowArtifact) error
	ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error)
	ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error)
	DeleteByTaskID(ctx context.Context, taskID string) error
}

// RepositoryRepository lists source code repositories for a project.
type RepositoryRepository interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error)
}

// ProjectRepository retrieves project configuration.
type ProjectRepository interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

// TaskRepository provides task CRUD operations.
type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
}

// WorkflowRepository manages workflow jobs, checkpoints, and logs.
type WorkflowRepository interface {
	Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	ClaimNext(ctx context.Context) (*models.WorkflowJob, error)
	LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	UpdateJob(ctx context.Context, jobID string, updates map[string]any) (*models.WorkflowJob, error)
	CreateCheckpoint(ctx context.Context, checkpoint models.WorkflowCheckpoint) error
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
	DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error
	DeleteByTaskID(ctx context.Context, taskID string) error
	CreateLog(ctx context.Context, log models.TaskLog) error
	ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error)
	ResetStuckJobs(ctx context.Context) error
	ResetAllRunningJobs(ctx context.Context) error
	AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error)
	ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error
}
