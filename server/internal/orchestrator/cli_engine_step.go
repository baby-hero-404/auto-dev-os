package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// cliCooldownDuration is how long a CLI credential is cooled down for after
// its output matches a quota/rate-limit signature (REQ-006 write-side).
// Matches gateway.go's capped transient-error cooldown for consistency; not
// configurable in this phase.
const cliCooldownDuration = 1 * time.Minute

// finishCLIRun performs the bookkeeping for the code_backend/code_frontend/
// fix dispatch path once RunCodeStep returns without a transport-level
// error: quota-cooldown write-back, auth-failure write-back, the finish log
// line, and saving the cli_output checkpoint artifact. Called by
// cliEngineRunner.RunLLMStep immediately after RunCodeStep succeeds, which
// then applies its own noop-check afterward. cliStepRunner.RunCLIStep
// (cli_analyze/cli_spec/cli_implement) inlines the equivalent logic itself
// instead (see cli_spec_step.go) since it needs differentiated success/
// failure log levels (REQ-001) that this shared helper doesn't provide.
func (o *Orchestrator) finishCLIRun(ctx context.Context, taskID, jobID, stepID, credID string, res *engine.CodeStepResult) {
	// Write-side of REQ-006: only affects the *next* ResolveExecutionProvider
	// call, not this step's own outcome (REQ-005, no mid-task switch).
	if res.QuotaExceeded && credID != "" && o.cooldownSetter != nil {
		_ = o.cooldownSetter.SetCooldown(ctx, credID, "", time.Now().Add(cliCooldownDuration))
	}

	// Auth-failure write-side: a bad session/token won't self-resolve like a
	// quota cooldown does, so mark the credential out instead of cooling it
	// down (see engine.CodeStepResult.AuthInvalid, cli_auth.go).
	if res.AuthInvalid && credID != "" && o.credStatusSetter != nil {
		_ = o.credStatusSetter.MarkNeedsReauth(ctx, credID)
	}

	o.log(ctx, taskID, &jobID, "info", fmt.Sprintf(
		"%s: cli engine finished (success=%v, exit_code=%d, output_bytes=%d)",
		stepID, res.Success, res.ExitCode, len(res.Output),
	))

	o.initCheckpoints()
	artifactBody := res.Output
	if artifactBody == "" {
		artifactBody = fmt.Sprintf("(cli produced no stdout/stderr; exit_code=%d)\ncommand: %s", res.ExitCode, res.Command)
	}
	_ = o.checkpoints.SaveArtifact(ctx, jobID, taskID, stepID, "cli_output", artifactBody)
}

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
	o      *Orchestrator
	cfg    *models.CLIEngineConfig
	orgID  string
	credID string
	eng    engine.ExecutionEngine
	once   sync.Once
	pErr   error
}

func newCLIEngineRunner(o *Orchestrator, cfg *models.CLIEngineConfig, orgID, credID string) *cliEngineRunner {
	return &cliEngineRunner{o: o, cfg: cfg, orgID: orgID, credID: credID, eng: engine.NewCLIEngine(o.runtime, o.credentials)}
}

func (r *cliEngineRunner) buildRequest(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (engine.CodeStepRequest, string, error) {
	r.o.initRepoutil()

	agentRole := ""
	if agent != nil {
		agentRole = agent.Role
	}
	resolvedRole := tool.EffectiveRoleForStep(stepID, agentRole, task)
	worktreeSuffix := worktreeSuffixForRole(resolvedRole)

	hostWorkspace := sandbox.WorkspacePath(r.o.workspaceRoot, task.ID)

	// The CLI must run from the actual repo checkout (or its role worktree),
	// not the bare task workspace root: hostWorkspace only bind-mounts to
	// /workspace, but the repo itself lives at a subpath under it
	// (code/repos/<name>/main, see repoutil.RepoHostPath). Passing
	// hostWorkspace straight into containerPathForHostPath used to always
	// collapse to "/workspace" (rel(localPath, hostWorkspace) == "."),
	// leaving the CLI cwd'd into a directory with no git repo in it.
	repoHostPath, err := r.o.repoutil.GetTaskRepoHostPath(ctx, task)
	if err != nil {
		return engine.CodeStepRequest{}, "", fmt.Errorf("cli engine: resolve repo path: %w", err)
	}
	worktreeHostPath := r.o.repoutil.HostWorktreePath(task, repoHostPath, worktreeSuffix)
	containerWorkDir := r.o.containerPathForHostPath(task, worktreeHostPath, "")

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
		OrgID:            r.orgID,
	}
	if r.cfg != nil && r.cfg.TimeoutMinutes > 0 {
		req.Timeout = time.Duration(r.cfg.TimeoutMinutes) * time.Minute
	}
	return req, worktreeSuffix, nil
}

// RunLLMStep implements steps.LLMRunner. Its return shape mirrors the
// agentic branch of runPatchRetryLoop's expectations: a non-empty
// parsed.summary marks the step as having applied real edits, which is all
// that path needs (see patch_retry_loop.go lines ~260-315) — the same
// targeted-test verification gate then applies regardless of which engine
// produced the edits.
func (r *cliEngineRunner) RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	req, worktreeSuffix, err := r.buildRequest(ctx, task, agent, jobID, stepID, instruction)
	if err != nil {
		return nil, fmt.Errorf("cli engine: %w", err)
	}

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
		r.o.log(ctx, task.ID, &jobID, "error", fmt.Sprintf("%s: cli engine run failed before producing a result: %v", stepID, err))
		return nil, fmt.Errorf("cli engine: %w", err)
	}

	r.o.finishCLIRun(ctx, task.ID, jobID, stepID, r.credID, res)

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
	resolved, err := o.ResolveExecutionProvider(ctx, task, project)
	if err != nil || resolved.Type != "cli" {
		return nil
	}
	if o.workflows != nil {
		cmdStr := resolved.CLIConfig.Command
		if len(resolved.CLIConfig.Args) > 0 {
			cmdStr += " " + strings.Join(resolved.CLIConfig.Args, " ")
		}
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf(
			"cli engine runner: execution provider resolved (ref=%q, credential_id=%q, cmd=%q)",
			resolved.Ref, resolved.CredentialID, cmdStr,
		))
	}
	return newCLIEngineRunner(o, resolved.CLIConfig, project.OrgID, resolved.CredentialID)
}
