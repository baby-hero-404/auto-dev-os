package steps

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestContextLoadStep_TransitionsStatusAndGathersContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-step-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{
		ID:        "task-123",
		ProjectID: "proj-123",
		Status:    models.TaskStatusTodo,
	}
	statusMock := &mockStatusUpdater{}
	artifactMock := &mockArtifactSaver{}
	sandboxMock := &mockSandboxRunner{
		result: StepResult{"stdout": "mock output\n"},
	}

	step := NewContextLoadStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		statusMock,
		&mockStepWorkspaceLoader{},
		sandboxMock,
		&mockContextEngine{},
		artifactMock,
		&mockRepositoryLister{},
		&mockLogger{},
		func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return "/sandbox/root"
		},
		nil,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !statusMock.called {
		t.Error("expected status updater to be called")
	}
	if statusMock.lastStatus != models.TaskStatusContextLoading {
		t.Errorf("expected transition to context loading, got: %s", statusMock.lastStatus)
	}
	if !artifactMock.called {
		t.Error("expected artifact to be saved")
	}

	gitLogs, ok := result["git_logs"].(map[string]string)
	if !ok || gitLogs["root"] != "mock output" {
		t.Errorf("expected git_logs to contain sandbox git output, got: %#v", gitLogs)
	}
}

type fakeLearnedSkillReader struct {
	skills []models.LearnedSkill
	err    error
}

func (f *fakeLearnedSkillReader) SearchActiveByText(ctx context.Context, projectID, query string, limit int) ([]models.LearnedSkill, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.skills) > limit {
		return f.skills[:limit], nil
	}
	return f.skills, nil
}

func TestContextLoadStep_LoadsLearnedSkillsWhenMatched(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-step-skills-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{ID: "task-1", ProjectID: "proj-1", Status: models.TaskStatusTodo}
	reader := &fakeLearnedSkillReader{skills: []models.LearnedSkill{
		{ID: "skill-1", Title: "Retry pattern", Content: "Use exponential backoff."},
	}}

	step := NewContextLoadStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		&mockStatusUpdater{},
		&mockStepWorkspaceLoader{},
		&mockSandboxRunner{result: StepResult{"stdout": "mock output\n"}},
		&mockContextEngine{},
		&mockArtifactSaver{},
		&mockRepositoryLister{},
		&mockLogger{},
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/sandbox/root" },
		reader,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	learnedSkills, ok := result["learned_skills"].(string)
	if !ok || learnedSkills == "" {
		t.Fatalf("expected learned_skills section to be set, got: %#v", result["learned_skills"])
	}
	if !strings.Contains(learnedSkills, "Retry pattern") {
		t.Errorf("expected matched skill title in rendered section, got: %s", learnedSkills)
	}

	ids, ok := result["skills_loaded"].([]string)
	if !ok || len(ids) != 1 || ids[0] != "skill-1" {
		t.Errorf("expected skills_loaded=[skill-1], got: %#v", result["skills_loaded"])
	}
}

func TestContextLoadStep_NoLearnedSkillsSectionWhenNoMatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "context-load-step-noskills-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{ID: "task-1", ProjectID: "proj-1", Status: models.TaskStatusTodo}
	reader := &fakeLearnedSkillReader{skills: nil}

	step := NewContextLoadStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		tmpDir,
		&mockTaskReader{task: task},
		&mockStatusUpdater{},
		&mockStepWorkspaceLoader{},
		&mockSandboxRunner{result: StepResult{"stdout": "mock output\n"}},
		&mockContextEngine{},
		&mockArtifactSaver{},
		&mockRepositoryLister{},
		&mockLogger{},
		func(task *models.Task, hostPath string, worktreeSuffix string) string { return "/sandbox/root" },
		reader,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["learned_skills"]; ok {
		t.Errorf("expected no learned_skills section when nothing matches, got: %#v", result["learned_skills"])
	}
}
