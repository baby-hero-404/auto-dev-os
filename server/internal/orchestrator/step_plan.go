package orchestrator

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepPlan(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := o.tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return map[string]any{"status": "skipped", "info": "skipped plan step for easy task"}, nil
	}
	var out map[string]any
	if o.llm != nil {
		out, err = o.runLLMStep(ctx, task, agent, jobID, workflow.StepPlan, "Create a concise JSON execution plan with subtasks, risks, and test strategy.")
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
	targetRepos, errRepos := o.loadTargetRepositories(ctx, task)
	if errRepos == nil && len(targetRepos) > 0 {
		integrationBranch := fmt.Sprintf("feature/%s", task.ID)
		beBranch := fmt.Sprintf("feature/%s-be", task.ID)
		feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

		ws, _ := o.LoadTaskWorkspace(ctx, task)
		o.setupRoleBranches(ctx, task, agent, jobID, targetRepos, ws)
		out["branches"] = map[string]string{
			"integration": integrationBranch,
			"backend":     beBranch,
			"frontend":    feBranch,
		}
	}

	if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusCoding); err != nil {
		return nil, err
	}
	return out, nil
}
