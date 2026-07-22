package steps

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockCLIStepRunner struct {
	output     CLIStepOutput
	err        error
	gotCapture []string
	gotInstr   string
}

func (m *mockCLIStepRunner) RunCLIStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string, captureFiles []string) (CLIStepOutput, error) {
	m.gotCapture = captureFiles
	m.gotInstr = instruction
	return m.output, m.err
}

type mockStepPromptLoader struct {
	prompt string
	err    error
}

func (m *mockStepPromptLoader) LoadStepPrompt(stepID string) (string, error) {
	return m.prompt, m.err
}

type mockCLITaskUpdater struct {
	updated models.UpdateTaskInput
	err     error
}

func (m *mockCLITaskUpdater) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	m.updated = input
	if m.err != nil {
		return nil, m.err
	}
	return &models.Task{ID: id}, nil
}

type mockCLILogger struct{}

func (m *mockCLILogger) Log(ctx context.Context, taskID string, jobID *string, level string, message string) {
}

func newCLITestTask() *models.Task {
	return &models.Task{ID: "task-1", ProjectID: "proj-1", Title: "Add feature", Description: "do the thing"}
}

func TestCLIAnalyzeStep_HappyPath(t *testing.T) {
	task := newCLITestTask()
	runner := &mockCLIStepRunner{output: CLIStepOutput{
		Files: map[string]string{
			cliAnalysisCapturePath: "## Tech Stack\n\nGo, React\n\n## Affected Files\n\n- a.go: does X\n- b.go\n\n## Risks\n\n- flaky test\n",
		},
	}}
	updater := &mockCLITaskUpdater{}
	step := NewCLIAnalyzeStep(StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}, updater, runner, &mockStepPromptLoader{prompt: "base prompt"}, &mockCLILogger{})

	res, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["status"] != "success" {
		t.Fatalf("expected success status, got %v", res)
	}
	if len(runner.gotCapture) != 1 || runner.gotCapture[0] != cliAnalysisCapturePath {
		t.Fatalf("expected capture of %s, got %v", cliAnalysisCapturePath, runner.gotCapture)
	}

	var payload cliAnalysisPayload
	if err := json.Unmarshal(updater.updated.Analysis, &payload); err != nil {
		t.Fatalf("failed to unmarshal saved analysis: %v", err)
	}
	if payload.TechStack != "Go, React" {
		t.Errorf("expected tech stack 'Go, React', got %q", payload.TechStack)
	}
	if len(payload.Files) != 2 || len(payload.Risks) != 1 {
		t.Errorf("expected 2 files and 1 risk, got %v / %v", payload.Files, payload.Risks)
	}
}

func TestCLIAnalyzeStep_MissingCapturedFile(t *testing.T) {
	task := newCLITestTask()
	runner := &mockCLIStepRunner{output: CLIStepOutput{Files: map[string]string{}}}
	step := NewCLIAnalyzeStep(StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}, &mockCLITaskUpdater{}, runner, &mockStepPromptLoader{prompt: "base"}, &mockCLILogger{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when analysis.md missing, got nil")
	}
}

func TestCLIAnalyzeStep_RunnerError(t *testing.T) {
	task := newCLITestTask()
	runner := &mockCLIStepRunner{err: errors.New("cli engine boom")}
	step := NewCLIAnalyzeStep(StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}, &mockCLITaskUpdater{}, runner, &mockStepPromptLoader{prompt: "base"}, &mockCLILogger{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error propagated from runner, got nil")
	}
}

func TestCLIAnalyzeStep_PromptLoadError(t *testing.T) {
	task := newCLITestTask()
	step := NewCLIAnalyzeStep(StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}, &mockCLITaskUpdater{}, &mockCLIStepRunner{}, &mockStepPromptLoader{err: errors.New("missing template")}, &mockCLILogger{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when prompt template fails to load, got nil")
	}
}
