package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string, jobStep string) map[string]workflow.StepFunc {
	deps := o.makeStepsDeps(task, agent, jobID)

	runners := map[string]workflow.StepFunc{
		workflow.StepContextLoad: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteContextLoad(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepAnalyze: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteAnalyze(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepPlan: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecutePlan(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepCodeBackend: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteCodeBackend(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepCodeFrontend: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteCodeFrontend(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepMerge: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteMerge(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepReview: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteReview(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepFix: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteFix(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepTest: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecuteTest(ctx, deps, task, agent, jobID, stepCtx)
		},
		workflow.StepPR: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			return steps.ExecutePR(ctx, deps, task, agent, jobID, stepCtx)
		},
	}

	for stepID, runner := range runners {
		runners[stepID] = o.withCheckpointRecovery(task, agent, jobStep, stepID, runner)
	}
	return runners
}

func (o *Orchestrator) makeStepsDeps(task *models.Task, agent *models.Agent, jobID string) *steps.Deps {
	o.initRepoutil()
	o.initCheckpoints()
	o.initWkspace()
	return &steps.Deps{
		Tasks:         o.tasks,
		Workflows:     o.workflows,
		Projects:      o.projects,
		Repos:         o.repositories,
		Agents:        o.agents,
		LLM:           o.llm,
		Prompts:       o.prompts,
		Runtime:       o.runtime,
		Wkspace:       o.wkspace,
		Checkpoints:   o.checkpoints,
		RepoUtil:      o.repoutil,
		SandboxGit:    o.sandboxGit,
		GitOps:        o.gitOps,
		Artifacts:     o.artifacts,
		WorkspaceRoot: o.workspaceRoot,

		RunLLMStep:               o.runLLMStep,
		RunSandboxStep:           o.runSandboxStep,
		RunSandboxStepInWorktree: o.runSandboxStepInWorktree,
		RunTargetedTests:         o.runTargetedTests,
		SaveArtifact:             o.saveArtifact,
		UpdateTaskStatus:         o.updateTaskStatus,
		Log:                      o.log,
		ContainerPathForHostPath: func(t *models.Task, hostPath string, worktreeSuffix string) string {
			return o.containerPathForHostPath(t, hostPath, worktreeSuffix)
		},
		ReadAffectedFileContent: o.readAffectedFileContent,
		WriteLLMCallTrace:       o.writeLLMCallTrace,
	}
}
