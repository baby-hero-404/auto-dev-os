package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// cliSpecRequiredFiles are the 4 OpenSpec files cli_spec must produce, in
// the convention this repo itself uses (docs/openspecs/<task-slug>/*.md).
var cliSpecRequiredFiles = []string{"proposal.md", "specs.md", "design.md", "tasks.md"}

// cliSpecAwaitingApprovalReason is reused verbatim from the analyze step's
// existing pause convention (analyze.go) so the frontend's existing
// paused-banner classification keeps working without changes.
const cliSpecAwaitingApprovalReason = "workflow paused for human spec review"

// CLISpecStep implements Step for the OpenSpec-authoring stage of the CLI
// spec-first flow.
type CLISpecStep struct {
	rt       StepRuntime
	worktree WorktreeHostPathResolver
	runner   CLIStepRunner
	prompts  StepPromptLoader
	log      Logger
	tasks    TaskUpdater
	projects ProjectReader
}

func NewCLISpecStep(rt StepRuntime, worktree WorktreeHostPathResolver, runner CLIStepRunner, prompts StepPromptLoader, log Logger, tasks TaskUpdater, projects ProjectReader) *CLISpecStep {
	return &CLISpecStep{rt: rt, worktree: worktree, runner: runner, prompts: prompts, log: log, tasks: tasks, projects: projects}
}

func (s *CLISpecStep) ID() string { return workflow.StepCLISpec }

func (s *CLISpecStep) StatusOnResume(_ StepResult) string { return models.TaskStatusAnalyzing }

// TaskSpecSlug returns the directory name cli_spec/cli_implement use under
// docs/openspecs/, matching the same slug convention as branch naming.
func TaskSpecSlug(task *models.Task) string {
	return paths.DeriveTaskSlug(task.ID, task.Title)
}

func (s *CLISpecStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	root, err := s.worktree.ResolveHostWorktreeRoot(ctx, s.rt.Task)
	if err != nil {
		return nil, fmt.Errorf("cli_spec: resolve worktree: %w", err)
	}
	slug := TaskSpecSlug(s.rt.Task)
	specDir := filepath.Join(root, "docs", "openspecs", slug)

	// Resume-after-approval: the reviewer already approved the spec on a
	// previous pass, so skip re-spawning the CLI and just re-validate the
	// files already committed to the worktree.
	if s.rt.Task.SpecStatus == models.TaskSpecStatusApproved {
		total, err := s.validateSpecFiles(specDir, slug)
		if err != nil {
			return nil, err
		}
		return StepResult{"status": "success", "spec_dir": specDir, "task_count": total}, nil
	}

	base, err := s.prompts.LoadStepPrompt(workflow.StepCLISpec)
	if err != nil {
		return nil, fmt.Errorf("cli_spec: load prompt: %w", err)
	}

	var analysisRaw string
	if len(s.rt.Task.Analysis) > 0 {
		var payload struct {
			RawMarkdown string `json:"raw_markdown"`
		}
		if json.Unmarshal(s.rt.Task.Analysis, &payload) == nil {
			analysisRaw = payload.RawMarkdown
		}
	}

	instruction := fmt.Sprintf(
		"%s\n\n## Task\n\n### %s\n\n%s\n\n## Task Slug\n\n%s\n\n## Analysis (from the previous step)\n\n%s\n",
		base, s.rt.Task.Title, s.rt.Task.Description, slug, analysisRaw,
	)

	if s.rt.Task.SpecStatus == models.TaskSpecStatusChangesRequested {
		instruction += fmt.Sprintf("\n## Reviewer feedback\n\n%s\n\nAddress this feedback and rewrite the 4 spec files accordingly.\n", s.rt.Task.Description)
	}

	if _, err := s.runner.RunCLIStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, s.ID(), instruction, nil); err != nil {
		return nil, fmt.Errorf("cli_spec: %w", err)
	}

	total, err := s.validateSpecFiles(specDir, slug)
	if err != nil {
		return nil, err
	}

	autonomy := models.AgentAutonomySupervised
	if s.projects != nil {
		if project, perr := s.projects.GetByID(ctx, s.rt.Task.ProjectID); perr == nil && project.DefaultAutonomy != "" {
			autonomy = project.DefaultAutonomy
		}
	}

	if autonomy == models.AgentAutonomyAutonomous {
		specStatus := models.TaskSpecStatusAutoApproved
		if s.tasks != nil {
			if _, err := s.tasks.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{SpecStatus: &specStatus}); err != nil {
				return nil, fmt.Errorf("cli_spec: persist auto-approval: %w", err)
			}
		}
		s.rt.Task.SpecStatus = specStatus
		return StepResult{"status": "success", "spec_dir": specDir, "task_count": total}, nil
	}

	specStatus := models.TaskSpecStatusPendingReview
	status := models.TaskStatusSpecReview
	if s.tasks != nil {
		if _, err := s.tasks.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{SpecStatus: &specStatus, Status: &status}); err != nil {
			return nil, fmt.Errorf("cli_spec: persist pending review: %w", err)
		}
	}
	s.rt.Task.SpecStatus = specStatus
	s.rt.Task.Status = status

	return nil, workflow.PauseError{Step: workflow.StepCLISpec, Reason: cliSpecAwaitingApprovalReason}
}

// validateSpecFiles checks all 4 required OpenSpec files exist in specDir
// and that tasks.md has at least one checkbox, returning the checkbox total.
func (s *CLISpecStep) validateSpecFiles(specDir, slug string) (int, error) {
	var missing []string
	var tasksMD string
	for _, name := range cliSpecRequiredFiles {
		content, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			missing = append(missing, name)
			continue
		}
		if name == "tasks.md" {
			tasksMD = string(content)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return 0, fmt.Errorf("cli_spec: missing required spec file(s) in docs/openspecs/%s/: %s", slug, strings.Join(missing, ", "))
	}

	_, total := workflow.ParseCheckboxes(tasksMD)
	if total == 0 {
		return 0, fmt.Errorf("cli_spec: tasks.md must contain at least one checkbox item")
	}
	return total, nil
}
