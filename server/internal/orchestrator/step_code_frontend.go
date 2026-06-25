package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepCodeFrontend(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := o.tasks.GetByID(ctx, task.ID)
	if err == nil {
		if t.Complexity == models.TaskComplexityEasy {
			return map[string]any{"status": "skipped", "info": "skipped frontend step for easy task"}, nil
		}
		var analysis models.TaskAnalysis
		if json.Unmarshal(t.Analysis, &analysis) == nil {
			hasFrontend := false
			for _, file := range analysis.AffectedFiles {
				if strings.HasPrefix(file, "web/") || strings.HasSuffix(file, ".tsx") || strings.HasSuffix(file, ".css") || strings.HasSuffix(file, ".html") {
					hasFrontend = true
					break
				}
			}
			if !hasFrontend {
				return map[string]any{"status": "skipped", "info": "no frontend files affected"}, nil
			}
		}
	}
	frontendAgent := agent
	if manager, ok := o.agents.(interface {
		AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
	}); ok {
		if fg, err := manager.AssignFrontendAgent(ctx, task); err == nil && fg != nil {
			frontendAgent = fg
			o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned frontend agent %s for frontend coding step", frontendAgent.Name))
		}
	}

	worktreeSuffix := ""
	if t != nil && t.Complexity != models.TaskComplexityEasy {
		worktreeSuffix = "-fe-worktree"
		if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
			ws, _ := o.LoadTaskWorkspace(ctx, task)
			if err := o.setupRoleWorktrees(ctx, task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
				return nil, err
			}
		}
	}

	if o.llm != nil {
		out, err := o.runLLMStep(ctx, task, frontendAgent, jobID, workflow.StepCodeFrontend, "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available.")
		if err != nil {
			return nil, err
		}
		if parsed, ok := out["parsed"].(map[string]any); ok {
			patch := patch.ExtractPatch(parsed)
			if patch != "" {
				_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "patch", patch)
				if applyErr := o.applyPatch(ctx, task, frontendAgent, workflow.StepCodeFrontend, patch, worktreeSuffix); applyErr != nil {
					return nil, fmt.Errorf("apply patch: %w", applyErr)
				}
			}
		}
		if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, frontendAgent, workflow.StepCodeFrontend, worktreeSuffix); diffErr == nil && diffText != "" {
			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "diff", diffText)
		}

		repoHostPath, err := o.getTaskRepoHostPath(ctx, task)
		if err != nil {
			return nil, err
		}

		changedFiles, diffErr := o.getChangedFiles(ctx, task, frontendAgent, repoHostPath, worktreeSuffix)
		if diffErr != nil {
			o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
		}

		if worktreeSuffix != "" {
			if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
				ws, _ := o.LoadTaskWorkspace(ctx, task)
				if err := o.commitRoleWorktrees(ctx, task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}

		if len(changedFiles) > 0 {
			if _, errT := o.runTargetedTests(ctx, task, frontendAgent, jobID, "code_frontend_test", changedFiles, worktreeSuffix); errT != nil {
				o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
