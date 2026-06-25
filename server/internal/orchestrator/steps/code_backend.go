package steps

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecuteCodeBackend(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	backendAgent := agent
	if manager, ok := deps.Agents.(interface {
		AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
	}); ok {
		if bg, err := manager.AssignBackendAgent(ctx, task); err == nil && bg != nil {
			backendAgent = bg
			deps.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned backend agent %s for backend coding step", backendAgent.Name))
		}
	}

	worktreeSuffix := ""
	t, _ := deps.Tasks.GetByID(ctx, task.ID)
	if t.Complexity != models.TaskComplexityEasy {
		worktreeSuffix = "-be-worktree"
		if deps.RepoUtil != nil {
			if targetRepos, err := deps.RepoUtil.LoadTargetRepositories(ctx, task); err == nil {
				var ws *models.TaskWorkspace
				if deps.Wkspace != nil {
					ws, _ = deps.Wkspace.LoadTaskWorkspace(ctx, task)
				}
				if err := deps.RepoUtil.SetupRoleWorktrees(ctx, task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}
	}

	if deps.LLM != nil {
		out, err := deps.RunLLMStep(ctx, task, backendAgent, jobID, workflow.StepCodeBackend, "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.")
		if err != nil {
			return nil, err
		}
		if parsed, ok := out["parsed"].(map[string]any); ok {
			p := patch.ExtractPatch(parsed)
			if p != "" {
				_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "patch", p)
				if deps.RepoUtil != nil {
					if applyErr := deps.RepoUtil.ApplyPatch(ctx, task, backendAgent, workflow.StepCodeBackend, p, worktreeSuffix); applyErr != nil {
						return nil, fmt.Errorf("apply patch: %w", applyErr)
					}
				}
			}
		}
		if deps.RepoUtil != nil {
			if diffText, diffErr := deps.RepoUtil.CaptureWorkspaceDiff(ctx, task, backendAgent, workflow.StepCodeBackend, worktreeSuffix); diffErr == nil && diffText != "" {
				_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "diff", diffText)
			}
		}

		var changedFiles []string
		if deps.RepoUtil != nil {
			repoHostPath, err := deps.RepoUtil.GetTaskRepoHostPath(ctx, task)
			if err != nil {
				return nil, err
			}

			var diffErr error
			changedFiles, diffErr = deps.RepoUtil.GetChangedFiles(ctx, task, backendAgent, repoHostPath, worktreeSuffix)
			if diffErr != nil {
				deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
			}
		}

		if worktreeSuffix != "" && deps.RepoUtil != nil {
			if targetRepos, err := deps.RepoUtil.LoadTargetRepositories(ctx, task); err == nil {
				var ws *models.TaskWorkspace
				if deps.Wkspace != nil {
					ws, _ = deps.Wkspace.LoadTaskWorkspace(ctx, task)
				}
				if err := deps.RepoUtil.CommitRoleWorktrees(ctx, task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}

		if len(changedFiles) > 0 {
			if _, errT := deps.RunTargetedTests(ctx, task, backendAgent, jobID, "code_backend_test", changedFiles, worktreeSuffix); errT != nil {
				deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
