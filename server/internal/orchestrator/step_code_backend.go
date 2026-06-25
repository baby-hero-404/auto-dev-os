package orchestrator

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepCodeBackend(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	backendAgent := agent
	if manager, ok := o.agents.(interface {
		AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
	}); ok {
		if bg, err := manager.AssignBackendAgent(ctx, task); err == nil && bg != nil {
			backendAgent = bg
			o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned backend agent %s for backend coding step", backendAgent.Name))
		}
	}

	worktreeSuffix := ""
	t, _ := o.tasks.GetByID(ctx, task.ID)
	if t.Complexity != models.TaskComplexityEasy {
		worktreeSuffix = "-be-worktree"
		if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
			ws, _ := o.LoadTaskWorkspace(ctx, task)
			if err := o.setupRoleWorktrees(ctx, task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
				return nil, err
			}
		}
	}

	if o.llm != nil {
		out, err := o.runLLMStep(ctx, task, backendAgent, jobID, workflow.StepCodeBackend, "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.")
		if err != nil {
			return nil, err
		}
		if parsed, ok := out["parsed"].(map[string]any); ok {
			patch := patch.ExtractPatch(parsed)
			if patch != "" {
				_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "patch", patch)
				if applyErr := o.applyPatch(ctx, task, backendAgent, workflow.StepCodeBackend, patch, worktreeSuffix); applyErr != nil {
					return nil, fmt.Errorf("apply patch: %w", applyErr)
				}
			}
		}
		if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, backendAgent, workflow.StepCodeBackend, worktreeSuffix); diffErr == nil && diffText != "" {
			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "diff", diffText)
		}

		repoHostPath, err := o.getTaskRepoHostPath(ctx, task)
		if err != nil {
			return nil, err
		}

		changedFiles, diffErr := o.getChangedFiles(ctx, task, backendAgent, repoHostPath, worktreeSuffix)
		if diffErr != nil {
			o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
		}

		if worktreeSuffix != "" {
			if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
				ws, _ := o.LoadTaskWorkspace(ctx, task)
				if err := o.commitRoleWorktrees(ctx, task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}

		if len(changedFiles) > 0 {
			if _, errT := o.runTargetedTests(ctx, task, backendAgent, jobID, "code_backend_test", changedFiles, worktreeSuffix); errT != nil {
				o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
