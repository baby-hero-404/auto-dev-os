package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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
//
// A single cliStepRunner instance is shared across every step of the
// cli_analyze -> cli_spec -> cli_implement -> cross_review -> cli_mr
// workflow for one job run (see step_registry.go's cliSpecRunner), so
// Preflight is cached the same way cliEngineRunner caches it — otherwise
// auth_check_command (a real sandbox container run) would fire again on
// every single step instead of once per job. The cache is keyed on the
// resolved candidate (Ref+CredentialID): if a credential hits quota
// cooldown mid-workflow and ResolveExecutionProvider falls back to a
// different CLI candidate for a later step, that new candidate's binary
// has never had Preflight run against it, so the cache must miss and
// re-run rather than reuse the previous candidate's cached result.
type cliStepRunner struct {
	o             *Orchestrator
	mu            sync.Mutex
	preflightKey  string
	preflightDone bool
	preflight     error
	credID        string // set by resolveConfig, read by RunCLIStep after RunCodeStep for cooldown write-back
	candidateKey  string // set by resolveConfig, read by RunCLIStep to key the preflight cache
}

func newCLIStepRunner(o *Orchestrator) *cliStepRunner {
	return &cliStepRunner{o: o}
}

// resolveConfig routes through the same Execution Router code_backend/
// code_frontend/fix already use (see execution_router.go) instead of
// reading project.CLIEngineConfig directly — otherwise a project configured
// only via the newer ExecutionProviders list never gets a usable config
// here, and quota-exceeded cooldown (REQ-006 write-side) never fires for
// the whole cli_analyze/cli_spec/cli_implement flow since no credential ID
// was ever threaded through.
func (r *cliStepRunner) resolveConfig(ctx context.Context, task *models.Task) (*models.CLIEngineConfig, string, error) {
	if r.o.projects == nil {
		return nil, "", fmt.Errorf("cli step runner: project repository unavailable")
	}
	project, err := r.o.projects.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil, "", fmt.Errorf("cli step runner: load project: %w", err)
	}
	resolved, err := r.o.ResolveExecutionProvider(ctx, task, project)
	if err != nil {
		return nil, "", fmt.Errorf("cli step runner: %w", err)
	}
	if resolved.Type != "cli" {
		// worker.go only builds a cliStepRunner when the Router has already
		// confirmed a cli candidate; landing here with type=="api" means the
		// two resolutions (workflow-shape selection, then this one) disagreed
		// — fail clearly instead of silently running with an empty config.
		return nil, "", fmt.Errorf("cli step runner: resolved provider is %q, not cli", resolved.Type)
	}
	r.credID = resolved.CredentialID
	r.candidateKey = resolved.Ref + "|" + resolved.CredentialID
	if r.o.workflows != nil {
		r.o.log(ctx, task.ID, nil, "info", fmt.Sprintf(
			"cli step runner: execution provider resolved (ref=%q, credential_id=%q)",
			resolved.Ref, resolved.CredentialID,
		))
	}
	return resolved.CLIConfig, project.OrgID, nil
}

func (r *cliStepRunner) RunCLIStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string, captureFiles []string, contextFiles map[string]string) (steps.CLIStepOutput, error) {
	cfg, orgID, err := r.resolveConfig(ctx, task)
	if err != nil {
		return steps.CLIStepOutput{}, err
	}

	r.o.initRepoutil()
	hostWorkspace := sandbox.WorkspacePath(r.o.workspaceRoot, task.ID)

	// Must resolve to the repo's actual checkout (code/repos/<name>/main),
	// not the bare task workspace root: passing hostWorkspace itself into
	// containerPathForHostPath always collapsed to "/workspace" (rel(local,
	// hostWorkspace) == "."), so the CLI agent ran with no git repo in its
	// cwd — see the empty-output/exit-1 cli_analyze failures this fixes.
	repoHostPath, err := r.o.repoutil.GetTaskRepoHostPath(ctx, task)
	if err != nil {
		return steps.CLIStepOutput{}, fmt.Errorf("cli step runner: resolve repo path: %w", err)
	}
	containerWorkDir := r.o.containerPathForHostPath(task, repoHostPath, "")

	networkMode := sandbox.NetworkModeNone
	if !r.o.disableNetworking {
		networkMode = sandbox.NetworkModeBridge
	}

	eng := engine.NewCLIEngine(r.o.runtime, r.o.credentials)
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
		ContextFiles:     contextFiles,
		OrgID:            orgID,
	}
	if cfg.TimeoutMinutes > 0 {
		req.Timeout = time.Duration(cfg.TimeoutMinutes) * time.Minute
	}

	r.mu.Lock()
	if !r.preflightDone || r.preflightKey != r.candidateKey {
		warning, pErr := eng.Preflight(ctx, req)
		r.preflight = pErr
		r.preflightKey = r.candidateKey
		r.preflightDone = true
		if warning != "" {
			r.o.log(ctx, task.ID, &jobID, "warn", warning)
		}
	}
	preflightErr := r.preflight
	r.mu.Unlock()
	if preflightErr != nil {
		return steps.CLIStepOutput{}, fmt.Errorf("cli engine preflight failed: %w", preflightErr)
	}

	res, err := eng.RunCodeStep(ctx, req)
	if err != nil {
		r.o.log(ctx, task.ID, &jobID, "error", fmt.Sprintf("%s: cli engine run failed before producing a result: %v", stepID, err))
		return steps.CLIStepOutput{}, err
	}

	r.o.finishCLIRun(ctx, task.ID, jobID, stepID, r.credID, res)

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

		if stepID == "cli_spec" && r.o.workspaceRoot != "" {
			slug := steps.TaskSpecSlug(task)
			worktreeRoot := r.o.repoutil.HostWorktreePath(task, repoHostPath, "")
			srcDir := filepath.Join(worktreeRoot, "docs", "openspecs", slug)
			dstDir := filepath.Join(sandbox.WorkspacePath(r.o.workspaceRoot, task.ID), "specs")

			if stat, err := os.Stat(srcDir); err == nil && stat.IsDir() {
				_ = os.MkdirAll(dstDir, 0755)
				cmd := exec.CommandContext(ctx, "cp", "-r", srcDir+"/.", dstDir+"/")
				_ = cmd.Run()
			}
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
