package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// resumeSessionIDCtxKeyType is a typed context key for the CLI session-resume
// signal (REQ-003, cli-session-continuity) so it can't collide with a plain
// string key from another package, matching the prompts.StepInputsCtxKey
// convention used alongside it.
type resumeSessionIDCtxKeyType struct{}

var resumeSessionIDCtxKey = resumeSessionIDCtxKeyType{}

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string, jobStep string) map[string]workflow.StepFunc {
	o.initRepoutil()
	o.initCheckpoints()
	o.initWkspace()

	rt := steps.StepRuntime{Task: task, Agent: agent, JobID: jobID}

	prStep := steps.NewPRStep(
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
		o.attestations,            // AttestationSigner
	)
	cliSpecRunner := newCLIStepRunner(o)

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
			o.learnedSkills, // LearnedSkillReader
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
		prStep,
		steps.NewCLIAnalyzeStep(
			rt,
			o.tasks,                   // TaskUpdater
			cliSpecRunner,             // CLIStepRunner
			o.prompts,                 // StepPromptLoader
			loggerAdapter{log: o.log}, // Logger
		),
		steps.NewCLISpecStep(
			rt,
			cliSpecRunner,             // WorktreeHostPathResolver
			cliSpecRunner,             // CLIStepRunner
			o.prompts,                 // StepPromptLoader
			loggerAdapter{log: o.log}, // Logger
			o.tasks,                   // TaskUpdater
			o.projects,                // ProjectReader
		),
		steps.NewCLIImplementStep(
			rt,
			cliSpecRunner,             // WorktreeHostPathResolver
			o.repoutil,                // WorktreeManager (git checkpoint)
			cliSpecRunner,             // CLIStepRunner
			o.prompts,                 // StepPromptLoader
			loggerAdapter{log: o.log}, // Logger
			o.workflows,               // CheckpointLister
			o.tasks,                   // TaskUpdater
		),
		steps.NewCLIImplementBackendStep(
			rt,
			cliSpecRunner,             // WorktreeHostPathResolver
			o.repoutil,                // WorktreeManager
			o.wkspace,                 // WorkspaceLoader
			o.agents,                  // BackendAgentAssigner
			cliSpecRunner,             // CLIStepRunner
			o.prompts,                 // StepPromptLoader
			loggerAdapter{log: o.log}, // Logger
			o.workflows,               // CheckpointLister
			o.tasks,                   // TaskUpdater
		),
		steps.NewCLIImplementFrontendStep(
			rt,
			cliSpecRunner,             // WorktreeHostPathResolver
			o.repoutil,                // WorktreeManager
			o.wkspace,                 // WorkspaceLoader
			o.agents,                  // FrontendAgentAssigner
			cliSpecRunner,             // CLIStepRunner
			o.prompts,                 // StepPromptLoader
			loggerAdapter{log: o.log}, // Logger
			o.workflows,               // CheckpointLister
			o.tasks,                   // TaskUpdater
		),
		steps.NewCrossReviewStep(
			rt,
			o.tasks,       // TaskReader
			o.projects,    // ProjectReader
			cliSpecRunner, // CLIReviewRunner
			o.repoutil,    // DiffCapturer
			cliSpecRunner, // WorktreeHostPathResolver
			artifactSaverAdapter{save: o.checkpoints.SaveArtifact}, // ArtifactSaver
			o.checkpoints, // CheckpointReader
			o.workflows,   // CheckpointLister
			statusUpdaterAdapter{update: o.updateTaskStatus}, // StatusUpdater
			loggerAdapter{log: o.log},                        // Logger
		),
		steps.NewCLIMRStep(prStep),
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
			if val, ok := sc.Initial["resume_session_id"].(string); ok && val != "" {
				// Only inject the resume session ID if this is the step that was actually retried.
				// Otherwise, if this step succeeds and the workflow engine proceeds to the next step
				// within the same run, the next step would erroneously inherit the previous step's session.
				if targetStep, ok := sc.Initial["retried_step_id"].(string); ok && targetStep == stepID {
					ctx = context.WithValue(ctx, resumeSessionIDCtxKey, val)
				}
			}
			res, err := step.Execute(ctx, sc)
			return map[string]any(res), err
		}
		runners[stepID] = o.checkpoints.WithCheckpointRecovery(
			stepID, resolver, task, runner, o.updateTaskStatus,
		)
	}

	return runners
}

type affectedFileReaderAdapter struct {
	read func(ctx context.Context, task *models.Task, file string) (string, bool)
}

func (a affectedFileReaderAdapter) ReadAffectedFileContent(ctx context.Context, task *models.Task, file string) (string, bool) {
	return a.read(ctx, task, file)
}
