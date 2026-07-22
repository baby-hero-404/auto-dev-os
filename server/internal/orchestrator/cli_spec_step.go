package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// cliStepRunner implements steps.CLIStepRunner for cli_analyze/cli_spec/
// cli_implement. Unlike cliEngineRunner (used by code_backend/code_frontend/
// fix), it dispatches directly through the CLI engine with no patch-retry-
// loop or "zero changes = failure" assumptions baked in — each spec-first
// step validates its own file-based contract instead (see cli_analyze.go,
// cli_spec.go, cli_implement.go).
type cliStepRunner struct {
	o *Orchestrator
}

func newCLIStepRunner(o *Orchestrator) *cliStepRunner {
	return &cliStepRunner{o: o}
}

func (r *cliStepRunner) resolveConfig(ctx context.Context, task *models.Task) (*models.CLIEngineConfig, error) {
	if r.o.projects == nil {
		return nil, fmt.Errorf("cli step runner: project repository unavailable")
	}
	project, err := r.o.projects.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("cli step runner: load project: %w", err)
	}
	var cfg models.CLIEngineConfig
	if len(project.CLIEngineConfig) > 0 {
		if err := json.Unmarshal(project.CLIEngineConfig, &cfg); err != nil {
			return nil, fmt.Errorf("cli step runner: parse cli_engine_config: %w", err)
		}
	}
	return &cfg, nil
}

// RunCLIStep implements steps.CLIStepRunner.
func (r *cliStepRunner) RunCLIStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string, captureFiles []string) (steps.CLIStepOutput, error) {
	cfg, err := r.resolveConfig(ctx, task)
	if err != nil {
		return steps.CLIStepOutput{}, err
	}

	r.o.initRepoutil()
	hostWorkspace := sandbox.WorkspacePath(r.o.workspaceRoot, task.ID)
	containerWorkDir := r.o.containerPathForHostPath(task, hostWorkspace, "")

	networkMode := sandbox.NetworkModeNone
	if !r.o.disableNetworking {
		networkMode = sandbox.NetworkModeBridge
	}

	eng := engine.NewCLIEngine(r.o.runtime)
	req := engine.CodeStepRequest{
		Task:             task,
		Agent:            agent,
		StepID:           stepID,
		JobID:            jobID,
		Instruction:      instruction,
		HostWorkspace:    hostWorkspace,
		ContainerWorkDir: containerWorkDir,
		NetworkMode:      networkMode,
		CLIConfig:        cfg,
		CaptureFiles:     captureFiles,
	}
	if cfg.TimeoutMinutes > 0 {
		req.Timeout = time.Duration(cfg.TimeoutMinutes) * time.Minute
	}

	if warning, err := eng.Preflight(ctx, req); err != nil {
		return steps.CLIStepOutput{}, fmt.Errorf("cli engine preflight failed: %w", err)
	} else if warning != "" {
		r.o.log(ctx, task.ID, &jobID, "warn", warning)
	}

	res, err := eng.RunCodeStep(ctx, req)
	if err != nil {
		return steps.CLIStepOutput{}, err
	}

	r.o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("%s: cli engine finished (success=%v)", stepID, res.Success))
	if res.Output != "" {
		r.o.initCheckpoints()
		_ = r.o.checkpoints.SaveArtifact(ctx, jobID, task.ID, stepID, "cli_output", res.Output)
	}

	out := steps.CLIStepOutput{Output: res.Output, Files: res.Files}
	if !res.Success {
		errMsg := res.Error
		if errMsg == "" {
			errMsg = "cli engine: step failed"
		}
		return out, fmt.Errorf("%s", errMsg)
	}

	if repoHostPath, err := r.o.repoutil.GetTaskRepoHostPath(ctx, task); err == nil {
		if changed, cErr := r.o.repoutil.GetChangedFiles(ctx, task, agent, repoHostPath, ""); cErr == nil {
			out.ChangedFiles = changed
		}
	}

	return out, nil
}

// ResolveHostWorktreeRoot implements steps.WorktreeHostPathResolver: the
// repo-root host path a cli_spec/cli_implement step should read committed
// files back from (docs/openspecs/<slug>/*.md), as opposed to the ephemeral
// .autocode/ output that goes through CaptureFiles instead.
func (r *cliStepRunner) ResolveHostWorktreeRoot(ctx context.Context, task *models.Task) (string, error) {
	r.o.initRepoutil()
	repoPath, err := r.o.repoutil.GetTaskRepoHostPath(ctx, task)
	if err != nil {
		return "", err
	}
	return r.o.repoutil.HostWorktreePath(task, repoPath, ""), nil
}
