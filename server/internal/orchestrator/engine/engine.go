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

	// OrgID is the owning organization of Task's project. Only needed by
	// engines that look up org-scoped resources (e.g. the CLI engine
	// resolving CLIConfig.CredentialID against the credential store).
	OrgID string

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

	// ContextFiles maps a path relative to .autocode/context/ to its content.
	// Written by RunCodeStep alongside prompt.md, before the CLI subprocess
	// spawns; torn down by the same .autocode/ cleanup. Nil/empty means no
	// context/ directory is created at all (identical to today's behavior).
	ContextFiles map[string]string

	// WorktreeSuffix identifies which role worktree this step runs against
	// (e.g. models.WorktreeSuffixBackend/Frontend), or "" for the main
	// checkout. Only used to namespace the host log directory (see cli.go's
	// logsHostDir) so two role tracks running as separate, concurrent
	// containers for the same task never bind-mount the same host log
	// directory into their own mcp-context's fixed in-container log path —
	// that would let their unlocked, interleaved writes to
	// mcp-server.log/mcp-trace.jsonl corrupt each other.
	WorktreeSuffix string
}

// CodeStepResult is the outcome of running one coding step through an engine.
type CodeStepResult struct {
	Success bool
	Output  string
	Error   string

	// ExitCode is the subprocess's exit status. Always populated when the
	// sandbox run itself succeeded (i.e. Run didn't return an error), even
	// when Success is true, so callers can log it unconditionally.
	ExitCode int

	// Command is the fully-resolved, redacted invocation the CLI engine
	// executed (command + args, placeholders substituted) — kept separate
	// from Output so callers can log/persist it even when the subprocess
	// produced no stdout/stderr at all.
	Command string

	// LoopKilled is true when the CLI engine terminated the subprocess early
	// because its output was judged to be looping (see loopDetector), either
	// via the post-hoc check over the fully captured output or because the
	// sandbox runtime's own live in-stream detector already force-killed the
	// container mid-run (Phase 7, "Smart Idle Timeout & Loop Detection").
	LoopKilled bool

	// IdleTimeoutHit is true when the sandbox runtime force-killed the
	// container because no stdout/stderr activity was observed for the
	// configured idle timeout (Phase 7) — distinct from LoopKilled (which
	// fires on repeated output, not silence) and from a plain req.Timeout
	// expiry (which returns a hard sandbox.Run error, not a CodeStepResult).
	IdleTimeoutHit bool

	// QuotaExceeded is true when the captured output/exit code matched a
	// known quota/rate-limit signature for this CLI (see cli_quota.go,
	// REQ-006 write-side). The caller (cliEngineRunner.RunLLMStep) uses this
	// to cool down the credential that ran this step — it does not change
	// Success/Error, which are still decided purely by exit code + loop
	// detection.
	QuotaExceeded bool
	QuotaCooldown time.Duration

	// AuthInvalid is true when the captured output matched a known "not
	// authenticated" signature for this CLI at any confidence level (see
	// cli_auth.go). The caller always treats this as a failed step
	// (Success=false); AuthInvalidConfirmed decides whether it's further
	// treated as *permanent*.
	AuthInvalid bool

	// AuthInvalidConfirmed is true only when AuthInvalid matched a
	// profile-specific rule (engine.AuthInvalidConfirmed confidence, see
	// cli_auth.go) rather than the generic "*" fallback list (engine.
	// AuthInvalidSuspected). Only a confirmed match is treated as
	// permanent: the caller marks the linked credential as needing
	// re-login (a new non-Active status) so SelectCredential stops
	// auto-picking it until a human re-authenticates — mirrors
	// QuotaExceeded's cooldown write-side but for a failure that won't
	// self-resolve with time — and should not spend remaining retry
	// attempts on it, since the same credential will produce the same
	// failure every time until a human re-runs the CLI auth capture flow.
	// A merely suspected (fallback-only) match is left to fail and retry
	// like any other runtime error instead, since the generic patterns can
	// incidentally match legitimate output.
	AuthInvalidConfirmed bool

	// AwaitingInput is true when the CLI's last output line looks like it
	// was blocked waiting for a clarifying answer (see
	// cli_question_detect.go). The caller (RunCLIStep) turns this into a
	// workflow.PauseError instead of a plain failure (REQ-006) since no
	// stdin is ever attached to sandboxed CLI runs.
	AwaitingInput bool

	// Files holds the content of paths requested via CaptureFiles that were
	// present after the run (missing files are simply absent from the map).
	Files map[string]string

	// CredentialID is the resolved req.CLIConfig.CredentialID this run
	// actually attempted to use (may be empty — a misconfiguration/resolution
	// gap, not necessarily an error), surfaced so callers can log it
	// alongside Command/ExitCode when tracing "which credential ran this".
	CredentialID string

	// CredentialFilesResolved is the number of files resolveCredentialFiles
	// produced from the linked credential's payload (0 when CredentialID is
	// empty, credential lookup failed loudly, or the saved payload itself was
	// empty). A run with CredentialID set but CredentialFilesResolved==0
	// means the linked credential exists but mounted nothing — the CLI then
	// falls back to whatever host-session auto-mount (authDirs) is available,
	// which is the "Not logged in" failure mode this field is meant to catch.
	CredentialFilesResolved int

	// TelemetryOK, CostUSD, DurationMS, and TokensUsed are the parsed result
	// of scanning Output for a trailing --output-format json summary block
	// (Phase 6, "Telemetry Parsing" — see parseCLITelemetry). TelemetryOK is
	// false (and the three metrics left zero) when no recognizable telemetry
	// JSON was present — e.g. the provider doesn't support/enforce structured
	// output yet — so callers can skip persisting a spurious all-zero row
	// instead of assuming absence means zero cost.
	TelemetryOK bool
	CostUSD     float64
	DurationMS  int64
	TokensUsed  int64
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
