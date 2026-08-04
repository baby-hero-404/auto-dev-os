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
	output            CLIStepOutput
	err               error
	gotCapture        []string
	gotInstr          string
	gotWorktreeSuffix string
}

func (m *mockCLIStepRunner) RunCLIStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string, captureFiles []string, contextFiles map[string]string, worktreeSuffix string) (CLIStepOutput, error) {
	m.gotCapture = captureFiles
	m.gotInstr = instruction
	m.gotWorktreeSuffix = worktreeSuffix
	return m.output, m.err
}

type mockStepPromptLoader struct {
	prompt string
	err    error
}

func (m *mockStepPromptLoader) LoadStepPrompt(stepID string) (string, error) {
	return m.prompt, m.err
}

func (m *mockStepPromptLoader) MaterializeCLIContext(ctx context.Context, task models.Task, agent *models.Agent, stepID string) (map[string]string, error) {
	return nil, nil
}

func (m *mockStepPromptLoader) LoadRolePrompt(agent *models.Agent, task models.Task, stepID string) (string, error) {
	return "", nil
}

type mockWorktreeHostPathResolver struct {
	root string
	err  error
}

func (m *mockWorktreeHostPathResolver) ResolveHostWorktreeRoot(ctx context.Context, task *models.Task, worktreeSuffix string) (string, error) {
	return m.root, m.err
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

func TestCLIAnalyzeStep_PausesForClarification(t *testing.T) {
	task := newCLITestTask()
	runner := &mockCLIStepRunner{output: CLIStepOutput{Output: "Analyzing...\nProceed with deletion? (y/n)", AwaitingInput: true}}
	updater := &mockCLITaskUpdater{}
	step := NewCLIAnalyzeStep(StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"}, updater, runner, &mockStepPromptLoader{prompt: "base"}, &mockCLILogger{})

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	var pauseErr workflow.PauseError
	if !errors.As(err, &pauseErr) {
		t.Fatalf("expected workflow.PauseError, got: %v", err)
	}
	if pauseErr.Step != workflow.StepCLIAnalyze {
		t.Errorf("expected pause on cli_analyze step, got %q", pauseErr.Step)
	}
	if updater.updated.SpecStatus == nil || *updater.updated.SpecStatus != models.TaskSpecStatusClarificationRequired {
		t.Errorf("expected SpecStatus=clarification_required to be persisted, got %+v", updater.updated)
	}
	var rounds []models.ClarificationRound
	if err := json.Unmarshal(updater.updated.Clarifications, &rounds); err != nil {
		t.Fatalf("failed to unmarshal clarifications: %v", err)
	}
	if len(rounds) != 1 || len(rounds[0].Questions) != 1 || rounds[0].Questions[0] != "Proceed with deletion? (y/n)" {
		t.Errorf("expected 1 round with the extracted question, got %+v", rounds)
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
