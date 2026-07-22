package steps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockWorktreeHostPathResolver struct {
	root string
	err  error
}

func (m *mockWorktreeHostPathResolver) ResolveHostWorktreeRoot(ctx context.Context, task *models.Task) (string, error) {
	return m.root, m.err
}

func writeSpecFiles(t *testing.T, root, slug string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "openspecs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func TestCLISpecStep_HappyPath(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeSpecFiles(t, root, slug, map[string]string{
		"proposal.md": "# Proposal",
		"specs.md":    "# Specs",
		"design.md":   "# Design",
		"tasks.md":    "- [ ] task one\n- [x] task two\n",
	})

	step := NewCLISpecStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIStepRunner{},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		&mockCLITaskUpdater{},
		&mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomyAutonomous}},
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["task_count"] != 2 {
		t.Errorf("expected task_count 2, got %v", res["task_count"])
	}
}

func TestCLISpecStep_SupervisedPauses(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeSpecFiles(t, root, slug, map[string]string{
		"proposal.md": "# Proposal",
		"specs.md":    "# Specs",
		"design.md":   "# Design",
		"tasks.md":    "- [ ] task one\n",
	})

	taskUpdater := &mockCLITaskUpdater{}
	step := NewCLISpecStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIStepRunner{},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		taskUpdater,
		&mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomySupervised}},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	var pauseErr workflow.PauseError
	if !errors.As(err, &pauseErr) {
		t.Fatalf("expected workflow.PauseError, got %v", err)
	}
	if pauseErr.Reason != cliSpecAwaitingApprovalReason {
		t.Errorf("unexpected pause reason: %s", pauseErr.Reason)
	}
	if taskUpdater.updated.SpecStatus == nil || *taskUpdater.updated.SpecStatus != models.TaskSpecStatusPendingReview {
		t.Errorf("expected spec_status pending_review to be persisted, got %v", taskUpdater.updated.SpecStatus)
	}
	if taskUpdater.updated.Status == nil || *taskUpdater.updated.Status != models.TaskStatusSpecReview {
		t.Errorf("expected status spec_review to be persisted, got %v", taskUpdater.updated.Status)
	}
}

func TestCLISpecStep_ResumeAfterApprovalSkipsRunner(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	task.SpecStatus = models.TaskSpecStatusApproved
	slug := TaskSpecSlug(task)
	writeSpecFiles(t, root, slug, map[string]string{
		"proposal.md": "# Proposal",
		"specs.md":    "# Specs",
		"design.md":   "# Design",
		"tasks.md":    "- [ ] task one\n",
	})

	runner := &mockCLIStepRunner{err: errors.New("must not be called")}
	step := NewCLISpecStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		runner,
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		&mockCLITaskUpdater{},
		&mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomySupervised}},
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error on resume-after-approval: %v", err)
	}
	if res["task_count"] != 1 {
		t.Errorf("expected task_count 1, got %v", res["task_count"])
	}
}

func TestCLISpecStep_MissingRequiredFile(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeSpecFiles(t, root, slug, map[string]string{
		"proposal.md": "# Proposal",
		"specs.md":    "# Specs",
		// design.md and tasks.md missing
	})

	step := NewCLISpecStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIStepRunner{},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		&mockCLITaskUpdater{},
		&mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomyAutonomous}},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error for missing spec files, got nil")
	}
}

func TestCLISpecStep_TasksMDWithNoCheckboxes(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeSpecFiles(t, root, slug, map[string]string{
		"proposal.md": "# Proposal",
		"specs.md":    "# Specs",
		"design.md":   "# Design",
		"tasks.md":    "no checkboxes here",
	})

	step := NewCLISpecStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIStepRunner{},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		&mockCLITaskUpdater{},
		&mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomyAutonomous}},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when tasks.md has no checkboxes, got nil")
	}
}

func TestCLISpecStep_RunnerError(t *testing.T) {
	task := newCLITestTask()
	step := NewCLISpecStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: t.TempDir()},
		&mockCLIStepRunner{err: errors.New("boom")},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		&mockCLITaskUpdater{},
		&mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomyAutonomous}},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error propagated from runner, got nil")
	}
}
