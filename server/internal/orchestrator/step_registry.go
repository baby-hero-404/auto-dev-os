package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string, jobStep string) map[string]workflow.StepFunc {
	runners := map[string]workflow.StepFunc{
		workflow.StepContextLoad: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepContextLoad(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepAnalyze: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepAnalyze(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepPlan: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepPlan(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepCodeBackend: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepCodeBackend(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepCodeFrontend: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepCodeFrontend(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepMerge: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepMerge(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepReview: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepReview(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepFix: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepFix(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepTest: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepTest(ctx, task, agent, jobID, stepCtx)
		},
		workflow.StepPR: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return o.executeStepPR(ctx, task, agent, jobID, stepCtx)
		},
	}

	for stepID, runner := range runners {
		runners[stepID] = o.withCheckpointRecovery(task, agent, jobStep, stepID, runner)
	}
	return runners
}
