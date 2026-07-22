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

type mockCLIWorktreeManager struct {
	checkpointErr error
	checkpointed  bool
}

func (m *mockCLIWorktreeManager) LoadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error) {
	return nil, nil
}
func (m *mockCLIWorktreeManager) SetupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
}
func (m *mockCLIWorktreeManager) SetupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	return nil
}
func (m *mockCLIWorktreeManager) CommitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	return nil
}
func (m *mockCLIWorktreeManager) ResetRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, worktreeSuffix string) error {
	return nil
}
func (m *mockCLIWorktreeManager) CreateGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (*models.CheckpointResult, error) {
	m.checkpointed = true
	if m.checkpointErr != nil {
		return nil, m.checkpointErr
	}
	return &models.CheckpointResult{}, nil
}
func (m *mockCLIWorktreeManager) RestoreGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, commitHash string, worktreeSuffix string) error {
	return nil
}
func (m *mockCLIWorktreeManager) RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string {
	return ""
}

func writeProposal(t *testing.T, root, slug, content string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "openspecs", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write proposal.md: %v", err)
	}
}

func TestCLIImplementStep_HappyPath(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal")
	writeSpecFiles(t, root, slug, map[string]string{"tasks.md": "- [x] done one\n- [ ] todo two\n"})

	git := &mockCLIWorktreeManager{}
	step := NewCLIImplementStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		git,
		&mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"server/main.go"}}},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["checked_tasks"] != 1 || res["total_tasks"] != 2 {
		t.Errorf("expected 1/2 checkboxes, got %v/%v", res["checked_tasks"], res["total_tasks"])
	}
	if !git.checkpointed {
		t.Error("expected git checkpoint to be created")
	}
}

func TestCLIImplementStep_NoChangedFiles(t *testing.T) {
	task := newCLITestTask()
	step := NewCLIImplementStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: t.TempDir()},
		&mockCLIWorktreeManager{},
		&mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: nil}},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when no files changed, got nil")
	}
}

func TestCLIImplementStep_OnlySpecDiff_NotDocsOnly(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal (no frontmatter)")

	step := NewCLIImplementStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIWorktreeManager{},
		&mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"docs/openspecs/" + slug + "/proposal.md"}}},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error when only spec files changed and task isn't docs-only, got nil")
	}
}

func TestCLIImplementStep_DocsOnlyViaFrontmatter_AllowsSpecOnlyDiff(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "---\ntype: documentation\n---\n# Proposal")
	writeSpecFiles(t, root, slug, map[string]string{"tasks.md": "- [x] a\n"})

	step := NewCLIImplementStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIWorktreeManager{},
		&mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"docs/openspecs/" + slug + "/proposal.md"}}},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
	)

	res, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("expected docs-only frontmatter to allow spec-only diff, got error: %v", err)
	}
	if res["status"] != "success" {
		t.Errorf("expected success, got %v", res)
	}
}

func TestCLIImplementStep_DocsOnlyViaLabel_AllowsSpecOnlyDiff(t *testing.T) {
	root := t.TempDir()
	task := newCLITestTask()
	task.Labels = []string{"docs-only"}
	slug := TaskSpecSlug(task)
	writeProposal(t, root, slug, "# Proposal")

	step := NewCLIImplementStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: root},
		&mockCLIWorktreeManager{},
		&mockCLIStepRunner{output: CLIStepOutput{ChangedFiles: []string{"docs/openspecs/" + slug + "/proposal.md"}}},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("expected docs-only label to allow spec-only diff, got error: %v", err)
	}
}

func TestCLIImplementStep_RunnerError(t *testing.T) {
	task := newCLITestTask()
	step := NewCLIImplementStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockWorktreeHostPathResolver{root: t.TempDir()},
		&mockCLIWorktreeManager{},
		&mockCLIStepRunner{err: errors.New("cli crashed")},
		&mockStepPromptLoader{prompt: "base"},
		&mockCLILogger{},
		nil,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error propagated from runner, got nil")
	}
}
