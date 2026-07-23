package steps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockTaskRepository struct {
	task *models.Task
}

func (m *mockTaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return m.task, nil
}

func (m *mockTaskRepository) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	return m.task, nil
}

type mockContextEngine struct {
	indexFunc       func(ctx context.Context) error
	globalCacheDir  string
	builtCaches     []string
	initLocalCalled bool

	// getRepoMapCtx/retrieveContextCtx capture the ctx each call actually received, so tests can
	// assert provider.WorkspaceRootKey was set on it (REQ: RetrieveContext/GetRepoMap must see the
	// task's workspace root, not the original un-augmented ctx — see
	// TestStepContextLoad_Execute_PropagatesWorkspaceRootToRepoMapAndRetrieveContext).
	getRepoMapCtx      context.Context
	retrieveContextCtx context.Context
}

func (m *mockContextEngine) GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error) {
	m.getRepoMapCtx = ctx
	return "", nil
}

func (m *mockContextEngine) RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error) {
	m.retrieveContextCtx = ctx
	return nil, nil
}

func (m *mockContextEngine) IndexWorkspace(ctx context.Context) error {
	if m.indexFunc != nil {
		return m.indexFunc(ctx)
	}
	return nil
}

func (m *mockContextEngine) Close() error { return nil }

func (m *mockContextEngine) GetGlobalCacheDir() string {
	return m.globalCacheDir
}

func (m *mockContextEngine) BuildGlobalCache(repoAbsPath string, repoName string, commitHash string) error {
	m.builtCaches = append(m.builtCaches, repoAbsPath+":"+repoName+":"+commitHash)
	return nil
}

func (m *mockContextEngine) InitLocalCache(wsRoot string, repoCommits []provider.RepoCommitInfo) error {
	m.initLocalCalled = true
	return nil
}

func TestStepContextLoad_Execute(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-123",
	}

	agent := &models.Agent{
		ID: "agent-123",
	}

	workspaceRoot := filepath.Join(tmpDir, "workspaces")
	taskWorkspace := filepath.Join(workspaceRoot, task.ID)
	if err := os.MkdirAll(taskWorkspace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskWorkspace, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskWorkspace, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	indexCallCount := 0
	mockEngine := &mockContextEngine{
		indexFunc: func(ctx context.Context) error {
			indexCallCount++
			return nil
		},
	}

	buildStep := func() *ContextLoadStep {
		return NewContextLoadStep(
			StepRuntime{Task: task, Agent: agent, JobID: "job-123"},
			workspaceRoot,
			&mockTaskReader{task: task},
			statusUpdaterAdapter{update: func(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
				return task, nil
			}},
			&mockStepWorkspaceLoader{},
			sandboxRunnerAdapter{run: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error) {
				if strings.Contains(stepID, "get_git_commit") {
					return map[string]any{"stdout": "abc123commit"}, nil
				}
				if strings.Contains(stepID, "get_git_remote_origin") {
					return map[string]any{"stdout": ""}, nil
				}
				return map[string]any{"stdout": "mock output"}, nil
			}},
			mockEngine,
			artifactSaverAdapter{save: func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
				return nil
			}},
			&mockRepositoryLister{},
			&mockLogger{},
			func(task *models.Task, hostPath string, worktreeSuffix string) string {
				return hostPath
			},
		nil,
		)
	}

	res, err := buildStep().Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify IndexWorkspace was called exactly once
	if indexCallCount != 1 {
		t.Errorf("expected 1 call to IndexWorkspace, got %d", indexCallCount)
	}

	// Verify git logs, current branches and test commands are loaded correctly
	gitLogs, ok := res["git_logs"].(map[string]string)
	if !ok || gitLogs["root"] != "mock output" {
		t.Errorf("expected git_logs root to be 'mock output', got: %v", res["git_logs"])
	}

	currentBranches, ok := res["current_branches"].(map[string]string)
	if !ok || currentBranches["root"] != "mock output" {
		t.Errorf("expected current_branches root to be 'mock output', got: %v", res["current_branches"])
	}

	testCommands, ok := res["test_commands"].([]string)
	if !ok || len(testCommands) == 0 {
		t.Errorf("expected detected test commands, got: %v", res["test_commands"])
	}
}

// TestStepContextLoad_Execute_PropagatesWorkspaceRootToRepoMapAndRetrieveContext reproduces the
// bug traced from task b5f92863-39df-46ae-849a-f4781c0b9987: RetrieveContext and GetRepoMap were
// called with the original ctx (no provider.WorkspaceRootKey set), instead of the workspace-scoped
// ctx already built for IndexWorkspace. Provider.GetRepoMap treats a missing WorkspaceRootKey as
// "scanning the global root" and short-circuits to an empty result, so the repo map/semantic
// snippets silently came back empty — even though the underlying AST indexing pipeline worked
// fine — forcing the analyze step to fall back to reading files one at a time via tools instead of
// getting a repo map upfront.
func TestStepContextLoad_Execute_PropagatesWorkspaceRootToRepoMapAndRetrieveContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-workspaceroot-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{ID: "task-789", ProjectID: "proj-789"}
	agent := &models.Agent{ID: "agent-789"}

	workspaceRoot := filepath.Join(tmpDir, "workspaces")
	taskWorkspace := filepath.Join(workspaceRoot, task.ID)
	if err := os.MkdirAll(taskWorkspace, 0755); err != nil {
		t.Fatal(err)
	}

	mockEngine := &mockContextEngine{}

	step := NewContextLoadStep(
		StepRuntime{Task: task, Agent: agent, JobID: "job-789"},
		workspaceRoot,
		&mockTaskReader{task: task},
		statusUpdaterAdapter{update: func(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
			return task, nil
		}},
		&mockStepWorkspaceLoader{},
		sandboxRunnerAdapter{run: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error) {
			return map[string]any{"stdout": "mock output"}, nil
		}},
		mockEngine,
		artifactSaverAdapter{save: func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
			return nil
		}},
		&mockRepositoryLister{},
		&mockLogger{},
		func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return hostPath
		},
		nil,
	)

	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expectedRoot := taskWorkspace
	if mockEngine.retrieveContextCtx == nil {
		t.Fatal("RetrieveContext was never called")
	}
	if got, _ := mockEngine.retrieveContextCtx.Value(provider.WorkspaceRootKey).(string); got != expectedRoot {
		t.Errorf("RetrieveContext's ctx WorkspaceRootKey = %q, want %q", got, expectedRoot)
	}

	if mockEngine.getRepoMapCtx == nil {
		t.Fatal("GetRepoMap was never called")
	}
	if got, _ := mockEngine.getRepoMapCtx.Value(provider.WorkspaceRootKey).(string); got != expectedRoot {
		t.Errorf("GetRepoMap's ctx WorkspaceRootKey = %q, want %q", got, expectedRoot)
	}
}

type statusUpdaterAdapter struct {
	update func(ctx context.Context, taskID string, newStatus string) (*models.Task, error)
}

func (s statusUpdaterAdapter) UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
	return s.update(ctx, taskID, newStatus)
}

type artifactSaverAdapter struct {
	save func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error
}

func (a artifactSaverAdapter) SaveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	return a.save(ctx, jobID, taskID, step, artType, payload)
}

func TestStepContextLoad_Execute_CacheMissAndBuild(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-cache-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	globalCacheDir := filepath.Join(tmpDir, "global_cache")
	if err := os.MkdirAll(globalCacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	task := &models.Task{
		ID:        "task-456",
		ProjectID: "proj-456",
	}

	agent := &models.Agent{
		ID: "agent-456",
	}

	workspaceRoot := filepath.Join(tmpDir, "workspaces")
	taskWorkspace := filepath.Join(workspaceRoot, task.ID)
	// Create repo main path inside workspace
	repoPath := filepath.Join(taskWorkspace, "code", "repos", "repo-abc")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize dummy git repo to make git commands run successfully
	_, _ = runGitCmd(repoPath, "init")
	_, _ = runGitCmd(repoPath, "config", "user.email", "test@test.com")
	_, _ = runGitCmd(repoPath, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repoPath, "file.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _ = runGitCmd(repoPath, "add", ".")
	_, _ = runGitCmd(repoPath, "commit", "-m", "first commit")
	commitHash, err := runGitCmd(repoPath, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("failed to get commit hash: %v", err)
	}

	mockEngine := &mockContextEngine{
		globalCacheDir: globalCacheDir,
	}

	wsLoader := &mockStepWorkspaceLoader{
		loadFunc: func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
			return &models.TaskWorkspace{
				Root: taskWorkspace,
				Repos: []models.RepoWorkspace{
					{
						RepoID: "repo-abc-id",
						Name:   "repo-abc",
						Paths: models.RepoWorkspacePaths{
							Main: "code/repos/repo-abc",
						},
					},
				},
			}, nil
		},
	}

	step := NewContextLoadStep(
		StepRuntime{Task: task, Agent: agent, JobID: "job-456"},
		workspaceRoot,
		&mockTaskReader{task: task},
		statusUpdaterAdapter{update: func(ctx context.Context, taskID string, newStatus string) (*models.Task, error) {
			return task, nil
		}},
		wsLoader,
		sandboxRunnerAdapter{run: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error) {
			if strings.Contains(stepID, "get_git_commit") {
				return map[string]any{"stdout": commitHash}, nil
			}
			return map[string]any{"stdout": "mock output"}, nil
		}},
		mockEngine,
		artifactSaverAdapter{save: func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
			return nil
		}},
		&mockRepositoryLister{},
		&mockLogger{},
		func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return hostPath
		},
		nil,
	)

	_, err = step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify that the global cache build was called for the repo
	if len(mockEngine.builtCaches) != 1 {
		t.Errorf("expected 1 built cache call, got %d", len(mockEngine.builtCaches))
	} else {
		expected := repoPath + ":repo-abc:" + commitHash
		if mockEngine.builtCaches[0] != expected {
			t.Errorf("expected build cache payload: %q, got: %q", expected, mockEngine.builtCaches[0])
		}
	}

	// Verify that InitLocalCache was called
	if !mockEngine.initLocalCalled {
		t.Errorf("expected InitLocalCache to be called, but it was not")
	}
}
