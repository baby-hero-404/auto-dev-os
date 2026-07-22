package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gopkg.in/yaml.v3"
)

const cliDocsOnlyLabel = "docs-only"

// CLIImplementStep implements Step for the implementation stage of the CLI
// spec-first flow: implement against the approved spec set and validate the
// result via git diff (mirroring the "evaluate by diff" philosophy the
// pluggable execution engine already uses for code_backend/code_frontend).
type CLIImplementStep struct {
	rt          StepRuntime
	worktree    WorktreeHostPathResolver
	git         WorktreeManager
	runner      CLIStepRunner
	prompts     StepPromptLoader
	log         Logger
	checkpoints CheckpointLister
}

func NewCLIImplementStep(rt StepRuntime, worktree WorktreeHostPathResolver, git WorktreeManager, runner CLIStepRunner, prompts StepPromptLoader, log Logger, checkpoints CheckpointLister) *CLIImplementStep {
	return &CLIImplementStep{rt: rt, worktree: worktree, git: git, runner: runner, prompts: prompts, log: log, checkpoints: checkpoints}
}

func (s *CLIImplementStep) ID() string { return workflow.StepCLIImplement }

func (s *CLIImplementStep) StatusOnResume(_ StepResult) string { return models.TaskStatusCoding }

func (s *CLIImplementStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	base, err := s.prompts.LoadStepPrompt(workflow.StepCLIImplement)
	if err != nil {
		return nil, fmt.Errorf("cli_implement: load prompt: %w", err)
	}

	slug := TaskSpecSlug(s.rt.Task)
	specDir := fmt.Sprintf("docs/openspecs/%s", slug)
	instruction := fmt.Sprintf(
		"%s\n\n## Task\n\n### %s\n\n%s\n\n## Spec set location\n\n%s/\n",
		base, s.rt.Task.Title, s.rt.Task.Description, specDir,
	)
	if feedback := crossReviewFeedback(ctx, s.checkpoints, s.rt.Task.ID); feedback != "" {
		instruction += "\n\n## Reviewer feedback\n\n" + feedback
	}

	out, err := s.runner.RunCLIStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, s.ID(), instruction, nil)
	if err != nil {
		return nil, fmt.Errorf("cli_implement: %w", err)
	}

	if len(out.ChangedFiles) == 0 {
		return nil, fmt.Errorf("cli_implement: run completed but produced no file changes")
	}

	root, err := s.worktree.ResolveHostWorktreeRoot(ctx, s.rt.Task)
	if err != nil {
		return nil, fmt.Errorf("cli_implement: resolve worktree: %w", err)
	}

	docsOnly := isDocsOnlyTask(s.rt.Task) || proposalDeclaresDocumentationOnly(root, slug)
	if !docsOnly && !hasChangeOutsideOpenspecs(out.ChangedFiles) {
		return nil, fmt.Errorf("cli_implement: implement produced no code changes (only docs/openspecs/ was touched)")
	}

	if s.git != nil {
		if _, err := s.git.CreateGitCheckpoint(ctx, s.rt.Task, s.rt.Agent, s.ID(), ""); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("cli_implement: failed to create git checkpoint: %v", err))
		}
	}

	tasksMD, _ := os.ReadFile(filepath.Join(root, "docs", "openspecs", slug, "tasks.md"))
	done, total := workflow.ParseCheckboxes(string(tasksMD))

	return StepResult{"status": "success", "checked_tasks": done, "total_tasks": total}, nil
}

func isDocsOnlyTask(task *models.Task) bool {
	for _, label := range task.Labels {
		if strings.EqualFold(strings.TrimSpace(label), cliDocsOnlyLabel) {
			return true
		}
	}
	return false
}

// hasChangeOutsideOpenspecs reports whether any changed file lies outside
// docs/openspecs/ — i.e. real application code/docs changed, not just the
// spec set itself.
func hasChangeOutsideOpenspecs(changedFiles []string) bool {
	for _, f := range changedFiles {
		if !strings.Contains(f, "docs/openspecs/") {
			return true
		}
	}
	return false
}

// proposalDeclaresDocumentationOnly reads docs/openspecs/<slug>/proposal.md
// and checks for YAML frontmatter `type: documentation`, the escape hatch
// for tasks whose entire deliverable is the spec set itself (e.g. "analyze
// and write an OpenSpec for X, don't implement it").
func proposalDeclaresDocumentationOnly(worktreeRoot, slug string) bool {
	content, err := os.ReadFile(filepath.Join(worktreeRoot, "docs", "openspecs", slug, "proposal.md"))
	if err != nil {
		return false
	}
	text := string(content)
	if !strings.HasPrefix(strings.TrimSpace(text), "---") {
		return false
	}
	parts := strings.SplitN(text, "---", 3)
	if len(parts) < 3 {
		return false
	}
	var fm struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(fm.Type), "documentation")
}
