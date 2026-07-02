package wkspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type testMockRepositoriesRepo struct {
	repos []models.Repository
}

func (m *testMockRepositoriesRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return m.repos, nil
}

type testMockTasksRepo struct{}

func (m *testMockTasksRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return nil, nil
}

type testMockWorkflowsRepo struct{}

func (m *testMockWorkflowsRepo) ListCheckpoints(ctx context.Context, taskID string) ([]models.WorkflowCheckpoint, error) {
	return nil, nil
}
func (m *testMockWorkflowsRepo) AcquireAdvisoryLock(ctx context.Context, taskID string) (any, bool, error) {
	return nil, true, nil
}
func (m *testMockWorkflowsRepo) ReleaseAdvisoryLock(ctx context.Context, lockConn any, taskID string) error {
	return nil
}
func (m *testMockWorkflowsRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	return nil
}

func TestInitTaskWorkspace_SingleRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-ws-single-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-123"
	task := &models.Task{
		ID:           "task-single",
		ProjectID:    "proj-123",
		RepositoryID: &repoID,
		Title:        "Single Repo Task",
		Description:  "Do something",
		Status:       "coding",
		Complexity:   "easy",
		SpecStatus:   "approved",
		Labels:       []string{"go", "backend"},
	}

	repos := []models.Repository{
		{
			ID:        "repo-123",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/repo-123.git",
			Branch:    "main",
		},
		{
			ID:        "repo-456",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/repo-456.git",
			Branch:    "develop",
		},
	}

	manager := NewManager(
		&testMockTasksRepo{},
		&testMockWorkflowsRepo{},
		&testMockRepositoriesRepo{repos: repos},
		nil,
		nil,
		tmpDir,
		WorkspaceRetention{},
		nil,
		nil,
	)

	ws, err := manager.InitTaskWorkspace(context.Background(), task)
	if err != nil {
		t.Fatalf("InitTaskWorkspace failed: %v", err)
	}

	// Verify folders exist
	dirs := []string{
		ws.SpecsDir,
		ws.ContextDir,
		ws.ArtifactsDir,
		ws.LogsDir,
		ws.PRDir,
		filepath.Join(ws.ArtifactsDir, "checkpoints"),
		filepath.Join(ws.ArtifactsDir, "diffs"),
		filepath.Join(ws.ArtifactsDir, "tests"),
		filepath.Join(ws.LogsDir, "llm"),
	}
	for _, d := range dirs {
		if stat, err := os.Stat(d); err != nil || !stat.IsDir() {
			t.Errorf("expected directory %s to exist", d)
		}
	}

	// Verify task.json
	taskJSONPath := filepath.Join(ws.Root, "task.json")
	taskBytes, err := os.ReadFile(taskJSONPath)
	if err != nil {
		t.Fatalf("failed to read task.json: %v", err)
	}
	var taskSnap models.TaskStateSnapshot
	if err := json.Unmarshal(taskBytes, &taskSnap); err != nil {
		t.Fatalf("failed to unmarshal task.json: %v", err)
	}
	if taskSnap.TaskID != task.ID || taskSnap.Title != task.Title {
		t.Errorf("task snapshot mismatch: got %+v", taskSnap)
	}

	// Verify metadata.json has only the targeted repo
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := os.ReadFile(metaJSONPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}
	var meta models.TaskWorkspaceMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}
	if len(meta.Repos) != 1 {
		t.Errorf("expected 1 repo in metadata, got %d", len(meta.Repos))
	} else {
		repoWS := meta.Repos[0]
		if repoWS.RepoID != "repo-123" || repoWS.Name != "repo-123" {
			t.Errorf("unexpected repo workspace mapping: %+v", repoWS)
		}
		if repoWS.Paths.Main != filepath.Join("code", "repos", "repo-123", "main") {
			t.Errorf("unexpected main path: %s", repoWS.Paths.Main)
		}
	}
}

func TestInitTaskWorkspace_MultiRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-ws-multi-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:           "task-multi",
		ProjectID:    "proj-123",
		RepositoryID: nil, // Multi-repo
		Title:        "Multi Repo Task",
		Description:  "Do things in multiple repos",
		Status:       "coding",
		Complexity:   "medium",
		SpecStatus:   "approved",
	}

	repos := []models.Repository{
		{
			ID:        "repo-123",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/repo-123.git",
			Branch:    "main",
		},
		{
			ID:        "repo-456",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/repo-456.git",
			Branch:    "develop",
		},
	}

	manager := NewManager(
		&testMockTasksRepo{},
		&testMockWorkflowsRepo{},
		&testMockRepositoriesRepo{repos: repos},
		nil,
		nil,
		tmpDir,
		WorkspaceRetention{},
		nil,
		nil,
	)

	ws, err := manager.InitTaskWorkspace(context.Background(), task)
	if err != nil {
		t.Fatalf("InitTaskWorkspace failed: %v", err)
	}

	// Verify metadata.json contains both repos
	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	metaBytes, err := os.ReadFile(metaJSONPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}
	var meta models.TaskWorkspaceMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}
	if len(meta.Repos) != 2 {
		t.Errorf("expected 2 repos in metadata, got %d", len(meta.Repos))
	}
}

func TestLoadTaskWorkspace_BackwardCompatibility(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-ws-compat-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-123"
	task := &models.Task{
		ID:           "task-compat",
		ProjectID:    "proj-123",
		RepositoryID: &repoID,
		Title:        "Compat Task",
		Status:       "coding",
	}

	repos := []models.Repository{
		{
			ID:        "repo-123",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/repo-123.git",
			Branch:    "main",
		},
	}

	manager := NewManager(
		&testMockTasksRepo{},
		&testMockWorkflowsRepo{},
		&testMockRepositoriesRepo{repos: repos},
		nil,
		nil,
		tmpDir,
		WorkspaceRetention{},
		nil,
		nil,
	)

	// Manually create the workspace folder, but NO metadata.json (simulating old task)
	wsPath := filepath.Join(tmpDir, task.ID)
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		t.Fatalf("failed to create fake existing workspace: %v", err)
	}

	// Verify LoadTaskWorkspace automatically recovers and initializes metadata.json
	ws, err := manager.LoadTaskWorkspace(context.Background(), task)
	if err != nil {
		t.Fatalf("LoadTaskWorkspace failed: %v", err)
	}

	metaJSONPath := filepath.Join(ws.Root, "metadata.json")
	if _, err := os.Stat(metaJSONPath); err != nil {
		t.Errorf("expected metadata.json to be regenerated, but got error: %v", err)
	}
	if len(ws.Repos) != 1 || ws.Repos[0].RepoID != "repo-123" {
		t.Errorf("unexpected loaded repo workspace mapping: %+v", ws.Repos)
	}
}

func TestFindRepoWorkspaceByPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-ws-find-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:        "task-find",
		ProjectID: "proj-123",
		Title:     "Find Repo Test",
		Status:    "coding",
	}

	repos := []models.Repository{
		{
			ID:        "repo-api",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/api.git",
			Branch:    "main",
		},
		{
			ID:        "repo-api-client",
			ProjectID: "proj-123",
			URL:       "git@github.com:org/api-client.git",
			Branch:    "main",
		},
	}

	manager := NewManager(
		&testMockTasksRepo{},
		&testMockWorkflowsRepo{},
		&testMockRepositoriesRepo{repos: repos},
		nil,
		nil,
		tmpDir,
		WorkspaceRetention{},
		nil,
		nil,
	)

	ws, err := manager.InitTaskWorkspace(context.Background(), task)
	if err != nil {
		t.Fatalf("InitTaskWorkspace failed: %v", err)
	}

	// 1. Check path inside api-client
	pathInsideApiClient := filepath.Join(ws.Root, "code", "repos", "api-client", "main", "src", "client.go")
	resolved, err := manager.FindRepoWorkspaceByPath(context.Background(), task, pathInsideApiClient)
	if err != nil {
		t.Fatalf("FindRepoWorkspaceByPath failed for api-client: %v", err)
	}
	if resolved.RepoID != "repo-api-client" {
		t.Errorf("expected repo-api-client, got %s", resolved.RepoID)
	}

	// 2. Check path inside api
	pathInsideApi := filepath.Join(ws.Root, "code", "repos", "api", "main", "src", "server.go")
	resolvedApi, err := manager.FindRepoWorkspaceByPath(context.Background(), task, pathInsideApi)
	if err != nil {
		t.Fatalf("FindRepoWorkspaceByPath failed for api: %v", err)
	}
	if resolvedApi.RepoID != "repo-api" {
		t.Errorf("expected repo-api, got %s", resolvedApi.RepoID)
	}
}
