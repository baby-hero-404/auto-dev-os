package steps

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecutePlan(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := deps.Tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return map[string]any{"status": "skipped", "info": "skipped plan step for easy task"}, nil
	}
	var out map[string]any
	if deps.LLM != nil {
		out, err = deps.RunLLMStep(ctx, task, agent, jobID, workflow.StepPlan, "Create a concise JSON execution plan with subtasks, risks, and test strategy.")
	} else {
		plan := []any{
			map[string]any{"id": "backend", "role": models.AgentRoleBackend, "description": "Implement server-side changes and data contracts."},
			map[string]any{"id": "frontend", "role": models.AgentRoleFrontend, "description": "Implement user-facing workflow updates when applicable."},
		}
		out, err = map[string]any{"subtasks": plan}, nil
	}
	if err != nil {
		return nil, err
	}

	// Allocate branches idempotently for medium/hard tasks
	if deps.RepoUtil != nil {
		targetRepos, errRepos := deps.RepoUtil.LoadTargetRepositories(ctx, task)
		if errRepos == nil && len(targetRepos) > 0 {
			integrationBranch := fmt.Sprintf("feature/%s", task.ID)
			beBranch := fmt.Sprintf("feature/%s-be", task.ID)
			feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

			var ws *models.TaskWorkspace
			if deps.Wkspace != nil {
				ws, _ = deps.Wkspace.LoadTaskWorkspace(ctx, task)
			}
			deps.RepoUtil.SetupRoleBranches(ctx, task, agent, jobID, targetRepos, ws)
			out["branches"] = map[string]string{
				"integration": integrationBranch,
				"backend":     beBranch,
				"frontend":    feBranch,
			}
		}
	}

	if _, err := deps.UpdateTaskStatus(ctx, task.ID, models.TaskStatusCoding); err != nil {
		return nil, err
	}
	return out, nil
}
