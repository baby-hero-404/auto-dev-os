// Package engine implements the pluggable code-execution engine abstraction:
// a project/task can run coding steps either through the built-in API-native
// LLM tool loop, or by spawning a generic, configurable CLI subprocess inside
// the existing sandbox container.
package engine

import (
	"context"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// CodeStepRequest carries everything an ExecutionEngine needs to run one
// coding step. Path resolution (host workspace vs. container worktree path)
// stays the caller's responsibility (orchestrator/repoutil already own that
// logic) so this package has no dependency on repoutil or docker internals.
type CodeStepRequest struct {
	Task    *models.Task
	Agent   *models.Agent
	StepID  string
	JobID   string
	Timeout time.Duration

	// Instruction is the fully-built coding instruction/prompt text, already
	// assembled by the caller (spec, frozen context, PR feedback, etc.).
	Instruction string

	// HostWorkspace is the host path bind-mounted into the sandbox container
	// (sandbox.CommandRequest.Workspace) — the task-level workspace root.
	HostWorkspace string

	// ContainerWorkDir is the path inside the container the CLI should run
	// from (typically the worktree for the acting role). May equal the
	// container root when there is no worktree split.
	ContainerWorkDir string

	// NetworkMode mirrors sandbox.CommandRequest.NetworkMode (e.g. "bridge"
	// or "none"); the caller decides this from project/org networking
	// policy, same as the existing sandbox step runners.
	NetworkMode string

	// CLIConfig is the resolved CLI engine configuration. Only used when the
	// resolved engine is "cli"; nil otherwise.
	CLIConfig *models.CLIEngineConfig

	// CaptureFiles lists paths, relative to ContainerWorkDir, whose content
	// should be captured and returned in CodeStepResult.Files. Needed for
	// files under the ephemeral .autocode/ directory, which the CLI engine
	// removes from the worktree immediately after the subprocess exits —
	// anything the caller needs to read back (e.g. an analysis report) must
	// be captured before that cleanup, not read from the host afterward.
	CaptureFiles []string
}

// CodeStepResult is the outcome of running one coding step through an engine.
type CodeStepResult struct {
	Success bool
	Output  string
	Error   string

	// LoopKilled is true when the CLI engine terminated the subprocess early
	// because its output was judged to be looping (see loopDetector).
	LoopKilled bool

	// Files holds the content of paths requested via CaptureFiles that were
	// present after the run (missing files are simply absent from the map).
	Files map[string]string
}

// ExecutionEngine abstracts over how a coding step actually gets executed.
type ExecutionEngine interface {
	// Name identifies the engine, e.g. "api_native" or "cli".
	Name() string

	// Preflight performs cheap, fast checks that the engine is usable before
	// committing to a real run (e.g. CLI binary present, auth valid). It
	// should return a descriptive error when the engine cannot run, and may
	// return a non-fatal warning (e.g. auth verification effectively
	// disabled) for the caller to log without blocking execution.
	Preflight(ctx context.Context, req CodeStepRequest) (warning string, err error)

	// RunCodeStep executes one coding step and returns its result.
	RunCodeStep(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error)
}

// ResolveEngine applies the precedence: task-level override > project
// default > api_native fallback.
func ResolveEngine(taskEngine *string, projectEngine string) string {
	if taskEngine != nil && *taskEngine != "" {
		return *taskEngine
	}
	if projectEngine != "" {
		return projectEngine
	}
	return models.ExecutionEngineAPINative
}
