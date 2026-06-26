package steps

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)


// PlanStep implements Step for the execution planning phase.
type PlanStep struct {
	rt        StepRuntime
	tasks     TaskReader
	llm       LLMRunner
	worktree  WorktreeManager
	workspace WorkspaceLoader
	status    StatusUpdater
	log       Logger
}

func NewPlanStep(
	rt StepRuntime,
	tasks TaskReader,
	llm LLMRunner,
	worktree WorktreeManager,
	workspace WorkspaceLoader,
	status StatusUpdater,
	log Logger,
) *PlanStep {
	return &PlanStep{
		rt:        rt,
		tasks:     tasks,
		llm:       llm,
		worktree:  worktree,
		workspace: workspace,
		status:    status,
		log:       log,
	}
}

func (s *PlanStep) ID() string                              { return workflow.StepPlan }
func (s *PlanStep) StatusOnResume(_ StepResult) string        { return models.TaskStatusCoding }

func (s *PlanStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	t, err := s.tasks.GetByID(ctx, s.rt.Task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return StepResult{"status": "skipped", "info": "skipped plan step for easy task"}, nil
	}
	var out StepResult
	if s.llm != nil {
		out, err = s.llm.RunLLMStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, workflow.StepPlan, "Create a concise JSON execution plan with subtasks, risks, and test strategy.")
	} else {
		plan := []any{
			map[string]any{"id": "backend", "role": models.AgentRoleBackend, "description": "Implement server-side changes and data contracts."},
			map[string]any{"id": "frontend", "role": models.AgentRoleFrontend, "description": "Implement user-facing workflow updates when applicable."},
		}
		out, err = StepResult{"subtasks": plan}, nil
	}
	if err != nil {
		return nil, err
	}

	// Allocate branches idempotently for medium/hard tasks
	if s.worktree != nil {
		targetRepos, errRepos := s.worktree.LoadTargetRepositories(ctx, s.rt.Task)
		if errRepos == nil && len(targetRepos) > 0 {
			integrationBranch := fmt.Sprintf("feature/%s", s.rt.Task.ID)
			beBranch := fmt.Sprintf("feature/%s-be", s.rt.Task.ID)
			feBranch := fmt.Sprintf("feature/%s-fe", s.rt.Task.ID)

			var ws *models.TaskWorkspace
			if s.workspace != nil {
				ws, _ = s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
			}
			s.worktree.SetupRoleBranches(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, targetRepos, ws)
			out["branches"] = map[string]string{
				"integration": integrationBranch,
				"backend":     beBranch,
				"frontend":    feBranch,
			}
		}
	}

	if s.status != nil {
		if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusCoding); err != nil {
			return nil, err
		}
	}
	return out, nil
}

