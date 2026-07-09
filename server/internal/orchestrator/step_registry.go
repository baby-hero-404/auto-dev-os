package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string, jobStep string) map[string]workflow.StepFunc {
	o.initRepoutil()
	o.initCheckpoints()
	o.initWkspace()

	rt := steps.StepRuntime{Task: task, Agent: agent, JobID: jobID}

	allSteps := []steps.Step{
		steps.NewContextLoadStep(
			rt,
			o.workspaceRoot,
			o.tasks, // TaskReader
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			o.wkspace, // WorkspaceLoader
			sandboxRunnerAdapter{run: o.runSandboxStep}, // SandboxRunner
			o.ctxEngine, // ContextEngine
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			o.repositories,            // RepositoryLister
			loggerAdapter{log: o.log}, // Logger
			o.containerPathForHostPath,
		),
		steps.NewAnalyzeStep(
			rt,
			o.workspaceRoot,
			o.tasks,    // TaskReader
			o.tasks,    // TaskUpdater
			o.projects, // ProjectReader
			o.llm,      // LLMChatter
			o.prompts,  // PromptAssembler
			sandboxRunnerAdapter{run: o.runSandboxStep},            // SandboxRunner
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			statusUpdaterAdapter{update: o.updateTaskStatus},       // StatusUpdater
			traceRecorderAdapter{write: o.writeLLMCallTrace},       // TraceRecorder
			loggerAdapter{log: o.log},                              // Logger
			o.wkspace,                                              // WorkspaceLoader
			o.containerPathForHostPath,
			o.maxPhaseCost,
		),
		steps.NewPlanStep(
			rt,
			o.tasks,                             // TaskReader
			llmRunnerAdapter{run: o.runLLMStep}, // LLMRunner
			o.repoutil,                          // WorktreeManager
			o.wkspace,                           // WorkspaceLoader
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			loggerAdapter{log: o.log},                        // Logger
			o.workflows,                         // WorkflowCheckpointRepo
			o.maxPhaseCost,                      // maxCost
		),
		steps.NewCodeBackendStep(
			rt,
			o.tasks,                             // TaskReader
			llmRunnerAdapter{run: o.runLLMStep}, // LLMRunner
			o.agents,                            // BackendAgentAssigner
			o.repoutil,                          // WorktreeManager
			o.repoutil,                          // DiffCapturer
			o.repoutil,                          // PatchApplier
			o.wkspace,                           // WorkspaceLoader
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			testerRunnerAdapter{run: o.runTargetedTests},           // TestRunner
			o.workflows,               // CheckpointLister
			loggerAdapter{log: o.log}, // Logger
		),
		steps.NewCodeFrontendStep(
			rt,
			o.tasks,                             // TaskReader
			llmRunnerAdapter{run: o.runLLMStep}, // LLMRunner
			o.agents,                            // FrontendAgentAssigner
			o.repoutil,                          // WorktreeManager
			o.repoutil,                          // DiffCapturer
			o.repoutil,                          // PatchApplier
			o.wkspace,                           // WorkspaceLoader
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			testerRunnerAdapter{run: o.runTargetedTests},           // TestRunner
			o.workflows,               // CheckpointLister
			loggerAdapter{log: o.log}, // Logger
		),
		steps.NewMergeStep(
			rt,
			o.tasks,      // TaskReader
			o.repoutil,   // WorktreeManager
			o.wkspace,    // WorkspaceLoader
			o.sandboxGit, // SandboxGitClient
			o.repoutil,   // DiffCapturer
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			statusUpdaterAdapter{update: o.updateTaskStatus},       // StatusUpdater
			o.containerPathForHostPath,
		),
		steps.NewReviewStep(
			rt,
			o.tasks,                             // TaskReader
			o.projects,                          // ProjectReader
			llmRunnerAdapter{run: o.runLLMStep}, // LLMRunner
			o.repoutil,                          // DiffCapturer
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			o.agents,      // ReviewerAssigner
			o.checkpoints, // CheckpointReader
			o.workflows,   // CheckpointLister
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			loggerAdapter{log: o.log},                        // Logger
		),
		steps.NewFixStep(
			rt,
			o.tasks,                             // TaskReader
			o.workflows,                         // CheckpointLister
			llmRunnerAdapter{run: o.runLLMStep}, // LLMRunner
			o.repoutil,                          // DiffCapturer
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			o.repoutil, // PatchApplier
			testerRunnerAdapter{run: o.runTargetedTests},     // TestRunner
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			loggerAdapter{log: o.log},                        // Logger
		),
		steps.NewTestStep(
			rt,
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			sandboxRunnerAdapter{run: o.runSandboxStep},      // SandboxRunner
			o.wkspace,     // WorkspaceLoader
			o.projects,    // ProjectReader
			o.checkpoints, // CheckpointReader
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			loggerAdapter{log: o.log},                              // Logger
		),
		steps.NewPRStep(
			rt,
			o.tasks, // TaskRepository
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			o.repoutil,   // WorktreeManager
			o.wkspace,    // WorkspaceLoader
			o.sandboxGit, // SandboxGitClient
			o.repoutil,   // DiffCapturer
			o.artifacts,  // ArtifactRepository
			o.projects,   // ProjectReader
			o.workflows,  // CheckpointLister
			o.gitOps,     // GitOpsClient
			o.containerPathForHostPath,
			loggerAdapter{log: o.log}, // Logger
		),
	}

	runners := make(map[string]workflow.StepFunc)
	for _, s := range allSteps {
		step := s // capture
		stepID := step.ID()
		resolver := func(output map[string]any) string {
			return step.StatusOnResume(steps.StepResult(output))
		}
		runner := func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
			ctx = context.WithValue(ctx, prompts.StepInputsCtxKey, sc.Inputs)
			res, err := step.Execute(ctx, sc)
			return map[string]any(res), err
		}
		runners[stepID] = o.checkpoints.WithCheckpointRecovery(
			stepID, resolver, task, agent, jobStep, runner,
			o.repoutil.ApplyPatch, o.updateTaskStatus,
		)
	}

	return runners
}
