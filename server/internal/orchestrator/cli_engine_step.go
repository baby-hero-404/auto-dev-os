package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func worktreeSuffixForRole(role string) string {
	switch role {
	case models.AgentRoleBackend:
		return models.WorktreeSuffixBackend
	case models.AgentRoleFrontend:
		return models.WorktreeSuffixFrontend
	default:
		return ""
	}
}

// cliEngineRunner implements steps.LLMRunner by dispatching through the
// subprocess-CLI execution engine instead of the API-native LLM tool loop.
// It's only wired into code_backend/code_frontend/fix for tasks resolved to
// engine "cli" (see stepRunners). Preflight runs once per job via sync.Once
// so a repeated auth/binary check doesn't run on every patch-retry attempt.
type cliEngineRunner struct {
	o    *Orchestrator
	cfg  *models.CLIEngineConfig
	eng  engine.ExecutionEngine
	once sync.Once
	pErr error
}

func newCLIEngineRunner(o *Orchestrator, cfg *models.CLIEngineConfig) *cliEngineRunner {
	return &cliEngineRunner{o: o, cfg: cfg, eng: engine.NewCLIEngine(o.runtime)}
}

func (r *cliEngineRunner) buildRequest(task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (engine.CodeStepRequest, string) {
	r.o.initRepoutil()

	agentRole := ""
	if agent != nil {
		agentRole = agent.Role
	}
	resolvedRole := tool.EffectiveRoleForStep(stepID, agentRole, task)
	worktreeSuffix := worktreeSuffixForRole(resolvedRole)

	hostWorkspace := sandbox.WorkspacePath(r.o.workspaceRoot, task.ID)
	containerWorkDir := r.o.containerPathForHostPath(task, hostWorkspace, worktreeSuffix)

	networkMode := sandbox.NetworkModeNone
	if !r.o.disableNetworking {
		networkMode = sandbox.NetworkModeBridge
	}

	req := engine.CodeStepRequest{
		Task:             task,
		Agent:            agent,
		StepID:           stepID,
		JobID:            jobID,
		Instruction:      instruction,
		HostWorkspace:    hostWorkspace,
		ContainerWorkDir: containerWorkDir,
		NetworkMode:      networkMode,
		CLIConfig:        r.cfg,
	}
	if r.cfg != nil && r.cfg.TimeoutMinutes > 0 {
		req.Timeout = time.Duration(r.cfg.TimeoutMinutes) * time.Minute
	}
	return req, worktreeSuffix
}

// RunLLMStep implements steps.LLMRunner. Its return shape mirrors the
// agentic branch of runPatchRetryLoop's expectations: a non-empty
// parsed.summary marks the step as having applied real edits, which is all
// that path needs (see patch_retry_loop.go lines ~260-315) — the same
// targeted-test verification gate then applies regardless of which engine
// produced the edits.
func (r *cliEngineRunner) RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	req, worktreeSuffix := r.buildRequest(task, agent, jobID, stepID, instruction)

	r.once.Do(func() {
		warning, err := r.eng.Preflight(ctx, req)
		r.pErr = err
		if warning != "" {
			r.o.log(ctx, task.ID, &jobID, "warn", warning)
		}
	})
	if r.pErr != nil {
		return nil, fmt.Errorf("cli engine preflight failed: %w", r.pErr)
	}

	res, err := r.eng.RunCodeStep(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cli engine: %w", err)
	}

	r.o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("%s: cli engine finished (success=%v)", stepID, res.Success))
	if res.Output != "" {
		r.o.initCheckpoints()
		_ = r.o.checkpoints.SaveArtifact(ctx, jobID, task.ID, stepID, "cli_output", res.Output)
	}

	if !res.Success {
		if res.Error != "" {
			return nil, fmt.Errorf("cli engine: %s", res.Error)
		}
		return nil, fmt.Errorf("cli engine: step failed")
	}

	// By default a run producing zero file changes is treated as a failed
	// step (a "successful" CLI run that touched nothing is almost always a
	// misconfiguration or a no-op prompt). AllowNoop opts a config out of
	// this check for genuinely read-only/inspection use cases.
	if r.cfg == nil || !r.cfg.AllowNoop {
		r.o.initRepoutil()
		if repoHostPath, err := r.o.repoutil.GetTaskRepoHostPath(ctx, task); err == nil {
			changedFiles, diffErr := r.o.repoutil.GetChangedFiles(ctx, task, agent, repoHostPath, worktreeSuffix)
			if diffErr == nil && len(changedFiles) == 0 {
				return nil, fmt.Errorf("cli engine: run completed but produced no file changes (set cli_engine_config.allow_noop to permit this)")
			}
		}
	}

	return map[string]any{
		"parsed": map[string]any{"summary": "cli engine run completed"},
	}, nil
}

// resolveCLIEngineRunner resolves the project/task execution engine and, if
// it's "cli", returns an LLMRunner-compatible adapter dispatching through
// the CLI engine. Returns nil when the resolved engine is api_native (the
// default and only engine when no project config exists), so callers can
// fall back to the existing llmRunnerAdapter unchanged.
func (o *Orchestrator) resolveCLIEngineRunner(ctx context.Context, task *models.Task) *cliEngineRunner {
	if o.projects == nil {
		return nil
	}
	project, err := o.projects.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil
	}
	resolved := engine.ResolveEngine(task.ExecutionEngine, project.ExecutionEngine)
	if resolved != models.ExecutionEngineCLI {
		return nil
	}
	var cfg models.CLIEngineConfig
	if len(project.CLIEngineConfig) > 0 {
		_ = json.Unmarshal(project.CLIEngineConfig, &cfg)
	}
	return newCLIEngineRunner(o, &cfg)
}
