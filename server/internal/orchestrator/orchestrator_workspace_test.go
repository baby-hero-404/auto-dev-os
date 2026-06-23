package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type testMockRepositoriesRepo struct {
	repos []models.Repository
}

func (m *testMockRepositoriesRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return m.repos, nil
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

	orchestrator := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories:  &testMockRepositoriesRepo{repos: repos},
	}

	ws, err := orchestrator.InitTaskWorkspace(context.Background(), task)
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

	orchestrator := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories:  &testMockRepositoriesRepo{repos: repos},
	}

	ws, err := orchestrator.InitTaskWorkspace(context.Background(), task)
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

	orchestrator := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories:  &testMockRepositoriesRepo{repos: repos},
	}

	// Manually create the workspace folder, but NO metadata.json (simulating old task)
	wsPath := filepath.Join(tmpDir, task.ID)
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		t.Fatalf("failed to create fake existing workspace: %v", err)
	}

	// Verify LoadTaskWorkspace automatically recovers and initializes metadata.json
	ws, err := orchestrator.LoadTaskWorkspace(context.Background(), task)
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

func TestResolveRepoWorkspace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-ws-resolve-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-123"
	task := &models.Task{
		ID:           "task-resolve",
		ProjectID:    "proj-123",
		RepositoryID: &repoID,
		Title:        "Resolve Task",
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

	orchestrator := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories:  &testMockRepositoriesRepo{repos: repos},
	}

	_, err = orchestrator.InitTaskWorkspace(context.Background(), task)
	if err != nil {
		t.Fatalf("InitTaskWorkspace failed: %v", err)
	}

	repoWS, err := orchestrator.ResolveRepoWorkspace(context.Background(), task, "repo-123")
	if err != nil {
		t.Fatalf("ResolveRepoWorkspace failed: %v", err)
	}
	if repoWS.RepoID != "repo-123" || repoWS.Name != "repo-123" {
		t.Errorf("unexpected resolved repo workspace: %+v", repoWS)
	}

	_, err = orchestrator.ResolveRepoWorkspace(context.Background(), task, "non-existent")
	if err == nil {
		t.Error("expected ResolveRepoWorkspace to fail for non-existent repo")
	}
}

func TestOrchestrator_LLMCallTrace_And_Redaction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-ws-trace-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-123"
	task := &models.Task{
		ID:           "task-trace",
		ProjectID:    "proj-123",
		RepositoryID: &repoID,
		Title:        "Trace Task",
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

	orchestrator := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories:  &testMockRepositoriesRepo{repos: repos},
	}

	ws, err := orchestrator.InitTaskWorkspace(context.Background(), task)
	if err != nil {
		t.Fatalf("failed to init task workspace: %v", err)
	}

	// Reconstruct mock inputs
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	// Note: We use raw structure conversion since package import paths are identical.
	type Response struct {
		Content      string `json:"content"`
		Model        string `json:"model"`
		PromptTokens int    `json:"prompt_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}

	importMessages := []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{
		{Role: "user", Content: "hello github token ghp_111122223333444455556666777788889999"},
	}

	importResponse := struct {
		Content      string `json:"content"`
		Model        string `json:"model"`
		PromptTokens int    `json:"prompt_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}{
		Content:      "my openai key is sk-111122223333444455556666777788889999000011112222",
		Model:        "gpt-4",
		PromptTokens: 10,
		OutputTokens: 20,
	}

	// Call writeLLMCallTrace via reflection or casting since types are same package
	// Or we can convert our inline structs back to the package types:
	var msgList []llm.Message
	for _, m := range importMessages {
		msgList = append(msgList, llm.Message{Role: m.Role, Content: m.Content})
	}
	respObj := &llm.Response{
		Content:      importResponse.Content,
		Model:        importResponse.Model,
		PromptTokens: importResponse.PromptTokens,
		OutputTokens: importResponse.OutputTokens,
	}
	parsedMap := map[string]any{"secret_value": "sk-111122223333444455556666777788889999000011112222"}

	orchestrator.writeLLMCallTrace(context.Background(), task, &models.Agent{ID: "agent-1", Name: "Agent Alpha", Role: "coder"}, "test-step", msgList, respObj, parsedMap)

	// Check files created
	traceDir := filepath.Join(ws.Root, "logs", "llm", "test-step", "call-1")
	filesToCheck := []string{
		"request.json",
		"response.json",
		"prompt.md",
		"output.md",
		"parsed.json",
		"metadata.json",
	}

	for _, fName := range filesToCheck {
		fPath := filepath.Join(traceDir, fName)
		content, err := os.ReadFile(fPath)
		if err != nil {
			t.Fatalf("expected trace file %s to be created, got error: %v", fName, err)
		}

		// Verify secrets are redacted in file contents
		strContent := string(content)
		if strings.Contains(strContent, "ghp_") || strings.Contains(strContent, "sk-") {
			t.Errorf("file %s contains unredacted secrets: %s", fName, strContent)
		}
	}
}
