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
	GetByID(ctx context.Context, id string) (*models.Agent, error)
}

// SandboxGitClient manages git operations within the sandbox.
type SandboxGitClient interface {
	CheckoutBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error
	CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error
	HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) bool
	ResetSoft(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, target string) error
	MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) (string, error)
}

// PromptBuilder assembles LLM messages and tool definitions for agents.
type PromptBuilder interface {
	Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error)
	AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message, tools []llm.ToolDefinition) ([]llm.Message, []llm.ToolDefinition, error)
	ListAllSkills(ctx context.Context, task models.Task) ([]llm.ToolDefinition, error)
	// LoadStepPrompt loads a step's standalone instruction template — used
	// by the CLI spec-first steps (cli_analyze/cli_spec/cli_implement),
	// which build one full instruction per spawn rather than assembling a
	// multi-section tool-loop prompt.
	LoadStepPrompt(stepID string) (string, error)
	// MaterializeCLIContext builds the platform-knowledge file set (relevant
	// skills, learned skills, task rules) written into a CLI task's
	// .autocode/context/ directory. Used by the CLI spec-first steps.
	MaterializeCLIContext(ctx context.Context, task models.Task, agent *models.Agent, stepID string) (map[string]string, error)
	// LoadRolePrompt resolves the agent's role prompt exactly as the
	// API-native tool-loop assembler does. Used by cli_implement so a CLI
	// agent spawned under AgentRoleFrontend/AgentRoleBackend gets the same
	// role-specific instructions the API-native flow would produce.
	LoadRolePrompt(agent *models.Agent, task models.Task, stepID string) (string, error)
}

// GitOpsClient handles git operations (clone, branch, push, PR).
type GitOpsClient interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	// CloneForTask clones a repository resolving credentials from the linked GitAccount.
	// Use this instead of CloneRepo inside the orchestrator so the git account token
	// is never stored in the task/repo model and is always resolved at clone time.
	CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error)
	// TokenForRepoURL resolves the same read credential CloneForTask uses,
	// without cloning. Used to provision the read-only git credential
	// helper agents call mid-task (wkspace.writeGitCredentialHelper).
	TokenForRepoURL(ctx context.Context, repoURL string) (string, error)
	CreateBranch(ctx context.Context, localPath, repoURL, branchName string) error
	CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error
	CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
	MergePullRequest(ctx context.Context, repoURL, prURL string) error
	IsPullRequestMerged(ctx context.Context, repoURL, prURL string) (bool, error)
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
	SuggestSkillFromTask(ctx context.Context, task *models.Task, job *models.WorkflowJob)
	ExtractLearnedSkills(ctx context.Context, task *models.Task, autonomous bool)
	RecordSkillOutcome(ctx context.Context, skillIDs []string, success bool)
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
	ListAll(ctx context.Context) ([]models.Repository, error)
}

// ProjectRepository retrieves project configuration.
type ProjectRepository interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

// OrganizationRepository retrieves organization configuration — currently
// only used for DefaultExecutionProviders, the org-wide routing fallback
// consulted when a project has no execution_providers of its own (see
// execution_router.go's precedence chain).
type OrganizationRepository interface {
	GetByID(ctx context.Context, id string) (*models.Organization, error)
}

// LearnedSkillReader looks up active learned skills matching a query, for
// context_load to inject into a task's context (REQ-002).
type LearnedSkillReader interface {
	SearchActiveByText(ctx context.Context, projectID, query string, limit int) ([]models.LearnedSkill, error)
}

// TaskRepository provides task CRUD operations.
type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
	ListRecentByStatus(ctx context.Context, statuses []string, limit int) ([]models.Task, error)
	// ListSubTasks returns a decomposed parent's children (task-subtask-decomposition
	// dispatch/reduce; already-shipped repo method, see internal/repository/task.go).
	ListSubTasks(ctx context.Context, parentID string) ([]models.Task, error)
}

// TaskAttemptRepository records per-execution-attempt forensics (start/end
// time, exit status, tokens, cost) separately from the Task row itself
// (docs/openspecs/task-subtask-decomposition).
type TaskAttemptRepository interface {
	Create(ctx context.Context, taskID string, sandboxRef string) (*models.TaskAttempt, error)
	Finalize(ctx context.Context, id string, input models.FinalizeTaskAttemptInput) (*models.TaskAttempt, error)
	ListByTaskID(ctx context.Context, taskID string) ([]models.TaskAttempt, error)
	GetLatestByTaskID(ctx context.Context, taskID string) (*models.TaskAttempt, error)
	ListByTaskIDs(ctx context.Context, taskIDs []string) ([]models.TaskAttempt, error)
}

// WorkflowRepository manages workflow jobs, checkpoints, and logs.
type WorkflowRepository interface {
	Enqueue(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	ClaimNext(ctx context.Context) (*models.WorkflowJob, error)
	LatestByTaskID(ctx context.Context, taskID string) (*models.WorkflowJob, error)
	UpdateJob(ctx context.Context, jobID string, updates map[string]any) (*models.WorkflowJob, error)
	AccumulateJobTelemetry(ctx context.Context, jobID string, costUSD float64, durationMS, tokensUsed int64) error
	CreateCheckpoint(ctx context.Context, checkpoint models.WorkflowCheckpoint) error
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
	DeleteCheckpoints(ctx context.Context, taskID string, steps []string) error
	DeleteByTaskID(ctx context.Context, taskID string) error
	CreateLog(ctx context.Context, log models.TaskLog) error
	ListLogs(ctx context.Context, taskID string) ([]models.TaskLog, error)
	TailLogs(ctx context.Context, taskID string, n int) ([]models.TaskLog, error)
	SubscribeLogs(taskID string) chan models.TaskLog
	UnsubscribeLogs(taskID string, ch chan models.TaskLog)
	ResetStuckJobs(ctx context.Context) error
	WakeSleepingJobs(ctx context.Context) error
	ResetAllRunningJobs(ctx context.Context) error
	AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error)
	ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error
}
