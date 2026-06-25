package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecuteCodeFrontend(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := deps.Tasks.GetByID(ctx, task.ID)
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
	if manager, ok := deps.Agents.(interface {
		AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
	}); ok {
		if fg, err := manager.AssignFrontendAgent(ctx, task); err == nil && fg != nil {
			frontendAgent = fg
			deps.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned frontend agent %s for frontend coding step", frontendAgent.Name))
		}
	}

	worktreeSuffix := ""
	if t != nil && t.Complexity != models.TaskComplexityEasy {
		worktreeSuffix = "-fe-worktree"
		if deps.RepoUtil != nil {
			if targetRepos, err := deps.RepoUtil.LoadTargetRepositories(ctx, task); err == nil {
				var ws *models.TaskWorkspace
				if deps.Wkspace != nil {
					ws, _ = deps.Wkspace.LoadTaskWorkspace(ctx, task)
				}
				if err := deps.RepoUtil.SetupRoleWorktrees(ctx, task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}
	}

	if deps.LLM != nil {
		out, err := deps.RunLLMStep(ctx, task, frontendAgent, jobID, workflow.StepCodeFrontend, "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available.")
		if err != nil {
			return nil, err
		}
		if parsed, ok := out["parsed"].(map[string]any); ok {
			p := patch.ExtractPatch(parsed)
			if p != "" {
				_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "patch", p)
				if deps.RepoUtil != nil {
					if applyErr := deps.RepoUtil.ApplyPatch(ctx, task, frontendAgent, workflow.StepCodeFrontend, p, worktreeSuffix); applyErr != nil {
						return nil, fmt.Errorf("apply patch: %w", applyErr)
					}
				}
			}
		}
		if deps.RepoUtil != nil {
			if diffText, diffErr := deps.RepoUtil.CaptureWorkspaceDiff(ctx, task, frontendAgent, workflow.StepCodeFrontend, worktreeSuffix); diffErr == nil && diffText != "" {
				_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "diff", diffText)
			}
		}

		var changedFiles []string
		if deps.RepoUtil != nil {
			repoHostPath, err := deps.RepoUtil.GetTaskRepoHostPath(ctx, task)
			if err != nil {
				return nil, err
			}

			var diffErr error
			changedFiles, diffErr = deps.RepoUtil.GetChangedFiles(ctx, task, frontendAgent, repoHostPath, worktreeSuffix)
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
				if err := deps.RepoUtil.CommitRoleWorktrees(ctx, task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
					return nil, err
				}
			}
		}

		if len(changedFiles) > 0 {
			if _, errT := deps.RunTargetedTests(ctx, task, frontendAgent, jobID, "code_frontend_test", changedFiles, worktreeSuffix); errT != nil {
				deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
