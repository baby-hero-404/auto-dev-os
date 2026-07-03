package steps

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// --- Narrow interfaces derived from actual step usage ---

// TaskReader reads task state. Used by: all steps.
type TaskReader interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
}

// TaskUpdater writes task state. Used by: analyze, pr.
type TaskUpdater interface {
	Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
}

// StatusUpdater transitions task status with validation. Used by: most steps.
type StatusUpdater interface {
	UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error)
}

// ProjectReader reads project config. Used by: analyze, review, fix, test, pr.
type ProjectReader interface {
	GetByID(ctx context.Context, id string) (*models.Project, error)
}

// LLMRunner executes a single-shot LLM step. Used by: plan, code_be, code_fe, review, fix.
type LLMRunner interface {
	RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (StepResult, error)
}

// LLMChatter provides multi-turn LLM chat. Used by: analyze (tool loop), context_load (profiler).
type LLMChatter interface {
	Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error)
	ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error)
}

// PatchApplier applies and validates code patches. Used by: code_be, code_fe, fix.
type PatchApplier interface {
	Validate(ctx context.Context, task *models.Task, patchData string, worktreeSuffix string) []error
	ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchData string, worktreeSuffix string) error
}

// DiffCapturer captures workspace diffs. Used by: code_be, code_fe, fix, merge, pr.
type DiffCapturer interface {
	CaptureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error)
	CapturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error)
	GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error)
	GetTaskRepoHostPath(ctx context.Context, task *models.Task) (string, error)
}

// WorktreeManager manages git worktrees for parallel coding. Used by: code_be, code_fe, plan.
type WorktreeManager interface {
	LoadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error)
	SetupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool)
	SetupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error
	CommitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error
	RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string
}

// SandboxRunner executes commands in Docker sandbox. Used by: context_load, test, analyze.
type SandboxRunner interface {
	RunCommand(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (StepResult, error)
}

// TestRunner runs targeted tests on changed files. Used by: code_be, code_fe, fix.
type TestRunner interface {
	RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (StepResult, error)
}

// ArtifactSaver persists step artifacts. Used by: all steps.
type ArtifactSaver interface {
	SaveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error
}

// ArtifactLister reads stored artifacts. Used by: pr.
type ArtifactLister interface {
	ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error)
}

// Logger writes task logs. Used by: all steps.
type Logger interface {
	Log(ctx context.Context, taskID string, jobID *string, level string, message string)
}

// SandboxGitClient executes git operations inside sandbox containers.
// Used by: merge (CheckoutBranch, MergeBranch, HasBranch, CommitChanges),
//
//	pr (GetChangedFiles, CheckoutNewBranch, GetPRDiff).
//
// NOTE: PatchApplier and TestRunner adapters internally wrap
// RunSandboxStepInWorktree — steps do not call it directly.
type SandboxGitClient interface {
	CheckoutBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error
	CheckoutNewBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) error
	HasBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) bool
	MergeBranch(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, branch string) (string, error)
	CommitChanges(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, message string) error
	GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
	GetPRDiff(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error)
}

// WorkspaceLoader loads/saves workspace metadata. Used by: context_load, plan, code, merge, test, pr.
type WorkspaceLoader interface {
	LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error)
	SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error
}

// CheckpointReader counts completed step cycles. Used by: review, test.
type CheckpointReader interface {
	CountSuccessful(ctx context.Context, taskID string, step string) int
}

// CheckpointLister lists all checkpoints. Used by: fix (pr_rejection feedback).
type CheckpointLister interface {
	ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error)
}

// PromptAssembler builds LLM prompts. Used by: analyze, context_load.
type PromptAssembler interface {
	AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error)
}

// TraceRecorder writes LLM call traces. Used by: analyze.
type TraceRecorder interface {
	WriteLLMCallTrace(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed StepResult)
}

// ReviewerAssigner optionally assigns a specialized reviewer agent. Used by: review.
type ReviewerAssigner interface {
	AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// AgentReleaser releases a borrowed agent back to the pool.
type AgentReleaser interface {
	Release(ctx context.Context, agentID string) error
}

// BackendAssigner optionally assigns a specialized backend agent. Used by: code_be.
type BackendAssigner interface {
	AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// FrontendAssigner optionally assigns a specialized frontend agent. Used by: code_fe.
type FrontendAssigner interface {
	AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
}

// GitOpsClient handles remote git operations. Used by: pr.
type GitOpsClient interface {
	CommitAndPush(ctx context.Context, localPath, repoURL, branchName, message string, files map[string]string, agentRole string) error
	CreatePullRequest(ctx context.Context, repoURL, branchName, title, body string) (string, error)
}

// RepositoryLister lists project repositories. Used by: context_load.
type RepositoryLister interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error)
}

// TaskRepository reads and updates tasks. Used by: pr.
type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error)
}

// ArtifactRepository creates and lists artifacts. Used by: pr.
type ArtifactRepository interface {
	Create(ctx context.Context, artifact *models.WorkflowArtifact) error
	ListByJobID(ctx context.Context, jobID string) ([]models.WorkflowArtifact, error)
	ListByTaskID(ctx context.Context, taskID string) ([]models.WorkflowArtifact, error)
}

var analysisMu sync.Mutex

// updateTaskAnalysis updates task.Analysis concurrently-safe.
func updateTaskAnalysis(ctx context.Context, taskID string, tasks TaskRepository, rtTask *models.Task, updateFn func(*models.TaskAnalysis) bool) error {
	analysisMu.Lock()
	defer analysisMu.Unlock()

	// 1. Fetch fresh task from DB
	freshTask, err := tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	// 2. Unmarshal Analysis
	var analysis models.TaskAnalysis
	if len(freshTask.Analysis) > 0 {
		if err := json.Unmarshal(freshTask.Analysis, &analysis); err != nil {
			return err
		}
	}

	// 3. Apply the update
	if !updateFn(&analysis) {
		// No changes needed
		return nil
	}

	// 4. Marshal and Update
	newRaw, err := json.Marshal(analysis)
	if err != nil {
		return err
	}

	if _, err := tasks.Update(ctx, taskID, models.UpdateTaskInput{
		Analysis: newRaw,
	}); err != nil {
		return err
	}

	// 5. Update local cache
	rtTask.Analysis = newRaw
	return nil
}

func completeTaskSubtaskBlock(taskBlock string) (string, bool) {
	taskBlock = strings.TrimSpace(taskBlock)
	if taskBlock == "" {
		return "", false
	}

	lines := strings.Split(taskBlock, "\n")
	updated := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
			updated = true
		}
	}
	if !updated {
		return "", false
	}
	return strings.Join(lines, "\n"), true
}

func updateTaskSubtaskMarkdown(tasksMD string, taskBlock string) (string, bool) {
	taskBlock = strings.TrimSpace(taskBlock)
	if taskBlock == "" {
		return tasksMD, false
	}
	if !strings.Contains(taskBlock, "\n") && !strings.HasPrefix(taskBlock, "## ") {
		return tasksMD, false
	}
	if strings.Count(tasksMD, taskBlock) != 1 {
		return tasksMD, false
	}

	completedBlock, ok := completeTaskSubtaskBlock(taskBlock)
	if !ok {
		return tasksMD, false
	}

	if strings.Contains(tasksMD, taskBlock) {
		return strings.Replace(tasksMD, taskBlock, completedBlock, 1), true
	}

	return tasksMD, false
}

func isFrontendFile(file string) bool {
	return strings.HasPrefix(file, "web/") ||
		strings.HasPrefix(file, "frontend/") ||
		strings.HasPrefix(file, "src/") ||
		strings.HasSuffix(file, ".tsx") ||
		strings.HasSuffix(file, ".jsx") ||
		strings.HasSuffix(file, ".css") ||
		strings.HasSuffix(file, ".html") ||
		strings.HasSuffix(file, ".vue") ||
		strings.HasSuffix(file, ".scss") ||
		strings.HasSuffix(file, ".sass") ||
		strings.HasSuffix(file, ".svelte")
}
