package steps

import (
	"context"
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

	llmCallCount := 0
	llmProvider := &mockLLMProvider{
		chatFunc: func(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
			llmCallCount++
			return &llm.Response{
				Model:   "mock-model",
				Content: `{}`,
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

	res, err := buildStep().Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify NO LLM calls were made during context load step
	if llmCallCount != 0 {
		t.Errorf("expected 0 LLM calls, got %d", llmCallCount)
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
