package steps

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
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

type mockLLMProvider struct {
	chatFunc func(ctx context.Context, messages []llm.Message) (*llm.Response, error)
}

func (m *mockLLMProvider) Name() string { return "mock" }
func (m *mockLLMProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.chatFunc(ctx, messages)
}
func (m *mockLLMProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	return m.Chat(ctx, messages)
}

func TestSanitizeRepoURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"https://x-access-token:github_pat_12345@github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"http://user:password@gitlab.com/org/repo.git", "http://gitlab.com/org/repo.git"},
		{"git@github.com:org/repo.git", "git@github.com:org/repo.git"},
	}

	for _, tc := range tests {
		actual := sanitizeRepoURL(tc.input)
		if actual != tc.expected {
			t.Errorf("sanitizeRepoURL(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/org/repo.git", "github.com/org/repo"},
		{"http://github.com/org/repo.git", "github.com/org/repo"},
		{"git@github.com:org/repo.git", "github.com/org/repo"},
		{"git@github.com:org/repo", "github.com/org/repo"},
		{"https://token@github.com/org/repo.git", "github.com/org/repo"},
	}

	for _, tc := range tests {
		actual := normalizeRepoURL(tc.input)
		if actual != tc.expected {
			t.Errorf("normalizeRepoURL(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestGetRepoHash(t *testing.T) {
	h1 := getRepoHash("https://github.com/org/repo.git")
	h2 := getRepoHash("git@github.com:org/repo")
	if h1 != h2 {
		t.Errorf("expected same hash for equivalent URLs, got %q vs %q", h1, h2)
	}
}

func TestStepContextLoad_Caching(t *testing.T) {
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

	mockProfile := RepoProfile{
		RepoURL:     "github.com/org/repo",
		GeneratedAt: "2026-06-24T15:40:00Z",
		CommitHash:  "abc123commit",
		Architecture: RepoArchitecture{
			Summary: "Mock Architecture Summary",
			DirectoryStructure: map[string]string{
				"server": "Go server backend",
			},
		},
		Conventions: RepoConventions{
			Language:    "Go",
			Naming:      "camelCase",
			LinterRules: "golangci-lint default",
		},
	}
	profileBytes, _ := json.Marshal(mockProfile)

	llmCallCount := 0
	llmProvider := &mockLLMProvider{
		chatFunc: func(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
			llmCallCount++
			return &llm.Response{
				Model:   "mock-model",
				Content: string(profileBytes),
			}, nil
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
			llmProvider,
			artifactSaverAdapter{save: func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
				return nil
			}},
			&mockRepositoryLister{},
			&mockLogger{},
			func(task *models.Task, hostPath string, worktreeSuffix string) string {
				return hostPath
			},
		)
	}

	// First execution (Cache Miss)
	res, err := buildStep().Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}

	if llmCallCount != 1 {
		t.Errorf("expected 1 LLM call on cache miss, got %d", llmCallCount)
	}

	architectures, ok := res["architectures"].(map[string]string)
	if !ok || architectures["cached_profile"] == "" {
		t.Errorf("expected cached_profile in architectures, got: %v", res["architectures"])
	}

	// Verify cache file was written to correct location
	repoHash := getRepoHash(filepath.Base(taskWorkspace))
	cacheFile := filepath.Join(filepath.Dir(workspaceRoot), "repositories", repoHash, "profile.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Errorf("expected cache file at %q, but not found", cacheFile)
	}

	// Second execution (Cache Hit)
	res2, err := buildStep().Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}

	if llmCallCount != 1 {
		t.Errorf("expected NO extra LLM call on cache hit, got count %d", llmCallCount)
	}

	architectures2, ok := res2["architectures"].(map[string]string)
	if !ok || architectures2["cached_profile"] == "" {
		t.Errorf("expected cached_profile in architectures on cache hit")
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
