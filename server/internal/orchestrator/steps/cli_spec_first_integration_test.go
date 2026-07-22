package steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// fakeCLIEngineRunner simulates the pluggable CLI engine end-to-end for the
// integration test: it "writes" whatever files each step expects into a temp
// worktree, mimicking what a real CLI agent (Claude Code, etc.) would leave
// behind on disk, without spawning any subprocess or container.
type fakeCLIEngineRunner struct {
	t    *testing.T
	root string
	slug string
}

func (f *fakeCLIEngineRunner) RunCLIStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string, captureFiles []string) (CLIStepOutput, error) {
	switch stepID {
	case workflow.StepCLIAnalyze:
		return CLIStepOutput{
			Files: map[string]string{
				cliAnalysisCapturePath: "## Tech Stack\n\nGo\n\n## Affected Files\n\n- server/main.go\n\n## Risks\n\n- none\n",
			},
		}, nil

	case workflow.StepCLISpec:
		dir := filepath.Join(f.root, "docs", "openspecs", f.slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			f.t.Fatalf("mkdir spec dir: %v", err)
		}
		files := map[string]string{
			"proposal.md": "# Proposal\n\nWhy/What Changes",
			"specs.md":    "# Specs",
			"design.md":   "# Design",
			"tasks.md":    "- [x] implement thing\n- [ ] write docs\n",
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				f.t.Fatalf("write %s: %v", name, err)
			}
		}
		return CLIStepOutput{}, nil

	case workflow.StepCLIImplement:
		codeFile := filepath.Join(f.root, "server", "main.go")
		if err := os.MkdirAll(filepath.Dir(codeFile), 0o755); err != nil {
			f.t.Fatalf("mkdir code dir: %v", err)
		}
		if err := os.WriteFile(codeFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
			f.t.Fatalf("write main.go: %v", err)
		}
		return CLIStepOutput{ChangedFiles: []string{"server/main.go"}}, nil

	default:
		f.t.Fatalf("unexpected stepID dispatched to fake CLI engine runner: %s", stepID)
		return CLIStepOutput{}, nil
	}
}

type fakeWorktreeHostPathResolver struct{ root string }

func (f *fakeWorktreeHostPathResolver) ResolveHostWorktreeRoot(ctx context.Context, task *models.Task) (string, error) {
	return f.root, nil
}

// fakePRStepStandIn stands in for cli_mr's embedded PRStep dependency in this
// integration test: cli_mr's push/merge-request logic is already covered by
// pr_step tests, so here we only verify the workflow reaches and executes it.
type noopWorktreeManager struct{}

func (noopWorktreeManager) LoadTargetRepositories(ctx context.Context, task *models.Task) ([]models.Repository, error) {
	return nil, nil
}
func (noopWorktreeManager) SetupRoleBranches(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, repos []models.Repository, ws *models.TaskWorkspace, skipFE bool) {
}
func (noopWorktreeManager) SetupRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	return nil
}
func (noopWorktreeManager) CommitRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, repos []models.Repository, ws *models.TaskWorkspace, roleName string, roleLabel string, worktreeSuffix string) error {
	return nil
}
func (noopWorktreeManager) ResetRoleWorktrees(ctx context.Context, task *models.Task, agent *models.Agent, worktreeSuffix string) error {
	return nil
}
func (noopWorktreeManager) CreateGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (*models.CheckpointResult, error) {
	return &models.CheckpointResult{}, nil
}
func (noopWorktreeManager) RestoreGitCheckpoint(ctx context.Context, task *models.Task, agent *models.Agent, commitHash string, worktreeSuffix string) error {
	return nil
}
func (noopWorktreeManager) RepoHostPath(task *models.Task, ws *models.TaskWorkspace, repo models.Repository) string {
	return ""
}

// TestCLISpecFirstWorkflow_Integration drives all 4 CLI spec-first steps
// through the real workflow.Engine DAG, with a fake CLI engine runner that
// writes real files into a temp worktree — verifying the full
// analyze -> spec -> implement -> mr pipeline wires together correctly,
// without touching cli_mr's push/PR logic (out of scope here; PRStep is
// covered separately).
func TestCLISpecFirstWorkflow_Integration(t *testing.T) {
	root := t.TempDir()
	task := &models.Task{ID: "task-1", ProjectID: "proj-1", Title: "Add feature", Description: "do the thing"}
	slug := TaskSpecSlug(task)
	agent := &models.Agent{ID: "agent-1"}

	fakeRunner := &fakeCLIEngineRunner{t: t, root: root, slug: slug}
	resolver := &fakeWorktreeHostPathResolver{root: root}
	rt := StepRuntime{Task: task, Agent: agent, JobID: "job-1"}

	analyze := NewCLIAnalyzeStep(rt, &mockCLITaskUpdater{}, fakeRunner, &mockStepPromptLoader{prompt: "analyze base"}, &mockCLILogger{})
	spec := NewCLISpecStep(rt, resolver, fakeRunner, &mockStepPromptLoader{prompt: "spec base"}, &mockCLILogger{}, &mockCLITaskUpdater{}, &mockProjectReader{project: &models.Project{DefaultAutonomy: models.AgentAutonomyAutonomous}})
	implement := NewCLIImplementStep(rt, resolver, noopWorktreeManager{}, fakeRunner, &mockStepPromptLoader{prompt: "implement base"}, &mockCLILogger{})

	runners := map[string]workflow.StepFunc{
		workflow.StepCLIAnalyze: func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
			res, err := analyze.Execute(ctx, sc)
			return map[string]any(res), err
		},
		workflow.StepCLISpec: func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
			res, err := spec.Execute(ctx, sc)
			return map[string]any(res), err
		},
		workflow.StepCLIImplement: func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
			res, err := implement.Execute(ctx, sc)
			return map[string]any(res), err
		},
		// cli_mr's real logic (push+PR) is exercised by pr_step tests, not here.
		workflow.StepCLIMR: func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
			return map[string]any{"status": "success"}, nil
		},
	}

	def := workflow.CLISpecFirstWorkflow(runners)

	engine := &workflow.Engine{MaxParallel: 1}
	result, err := engine.Run(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("unexpected workflow error: %v", err)
	}

	for _, stepID := range []string{workflow.StepCLIAnalyze, workflow.StepCLISpec, workflow.StepCLIImplement, workflow.StepCLIMR} {
		if got := result.Status[stepID]; got != workflow.StepStatusSuccess {
			t.Errorf("step %s: expected status success, got %s", stepID, got)
		}
	}

	specDir := filepath.Join(root, "docs", "openspecs", slug)
	for _, name := range []string{"proposal.md", "specs.md", "design.md", "tasks.md"} {
		if _, err := os.Stat(filepath.Join(specDir, name)); err != nil {
			t.Errorf("expected %s to exist in spec dir: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "server", "main.go")); err != nil {
		t.Errorf("expected implement step to have written server/main.go: %v", err)
	}

	implOut := result.Outputs[workflow.StepCLIImplement]
	if implOut["checked_tasks"] != 1 || implOut["total_tasks"] != 2 {
		t.Errorf("expected 1/2 checked tasks from tasks.md, got %v/%v", implOut["checked_tasks"], implOut["total_tasks"])
	}
}
