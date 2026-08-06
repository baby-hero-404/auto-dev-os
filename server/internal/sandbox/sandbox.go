package sandbox

import (
	"context"
	"fmt"
	"io"
	"maps"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
)

const (
	NetworkModeDefault = ""
	NetworkModeNone    = "none"
	NetworkModeBridge  = "bridge"
)

// SandboxHomeDir is $HOME for the sandbox image's runtime user (the
// auto-code-os-sandbox image runs as USER "agent", not root — confirmed via
// `docker image inspect`). CLI credential injection (CredentialFiles) and
// the host-session auto-mount fallback (docker.go's authDirs) must target
// paths under here, not /root: mounting to /root succeeds silently, but a
// CLI process running as "agent" has $HOME=/home/agent and never looks
// there, so auth always appears missing even though the mount worked.
const SandboxHomeDir = "/home/agent"

type CommandRequest struct {
	TaskID      string
	AgentID     string
	Workspace   string
	Command     []string
	Env         map[string]string
	SecretEnv   map[string]string
	NetworkMode string
	Timeout     time.Duration
	// ResizeCh optionally delivers PTY size updates for RunInteractive
	// sessions (e.g. driven by the browser terminal's container size).
	// Runtimes that don't support TTY resize may ignore it.
	ResizeCh <-chan TerminalSize
	// CredentialFiles optionally materializes decrypted CLI credential
	// payloads into the container at specific absolute paths (e.g.
	// "/root/.claude.json" -> file content), keyed by target container path.
	// Runtimes that support it must bind-mount these read-write (some CLIs
	// refresh tokens on use) and must take priority over any other
	// convenience credential mount that would target the same path.
	CredentialFiles map[string]string

	// LogsHostDir, if set, is bind-mounted read-write at the fixed container
	// path LogsContainerDir (Phase 6, "Docker Log Bind-Mount"). This is where
	// mcp-context writes mcp-server.log/mcp-trace.jsonl from inside the
	// container, so they survive container removal and are visible on the
	// host outside the volatile code repository. Runtimes that don't support
	// bind mounts (e.g. StubRuntime) may ignore it.
	LogsHostDir string

	// SessionMounts defines explicit bind mounts (container absolute path -> host absolute path).
	// This is used by the orchestrator to map specific persistence directories (like ~/.claude or ~/.gemini)
	// to isolated session folders on the host, decoupling session continuity from the container's $HOME.
	SessionMounts map[string]string

	// LogFilePath, if set, is an absolute host path a supporting runtime
	// streams the subprocess's combined stdout/stderr into in real time
	// (io.MultiWriter alongside the buffered CommandResult.Stdout/Stderr) as
	// it runs, not just after exit — so a persistent log survives a mid-run
	// container kill (Phase 6, "Real-time Log Streaming"). Conventionally a
	// file under LogsHostDir, but the two are independent knobs.
	LogFilePath string

	// IdleTimeout, if > 0, overrides DefaultIdleTimeout (Phase 7, "Smart
	// Idle Timeout"): a supporting runtime kills the container if no
	// stdout/stderr byte is observed for this long, distinct from Timeout
	// (an absolute cap regardless of activity) — a long but genuinely
	// active run (e.g. a real 1.5h refactor that keeps printing progress)
	// never trips this, while a hung/silent process does long before
	// Timeout would fire. Zero means "use DefaultIdleTimeout", never
	// "disabled" — callers that truly want no idle enforcement (there are
	// none today) would need a distinct negative sentinel, not added since
	// unused. Runtimes without live log streaming (e.g. StubRuntime) ignore
	// it.
	IdleTimeout time.Duration

	// Image, if non-empty, overrides DockerConfig.Image for this single
	// request. Populated by SandboxManager once it has detected a project's
	// runtime and resolved its manifest's image tag — left empty by every
	// caller that builds a CommandRequest directly (unchanged today's
	// fixed-image behavior), so this field is additive, not a breaking
	// change to any existing call site.
	Image string

	// ExtraCacheMounts is an additional set of container-path -> host-path
	// bind mounts, merged on top of docker.go's hardcoded cacheDirs map, not
	// a replacement for it — replacing the hardcoded set would require
	// auditing every existing caller that relies on it implicitly. Populated
	// by SandboxManager from the detected runtime manifest's `cache:` list
	// (e.g. flutter's ~/.pub-cache). A runtime honoring this must skip
	// mounting an entry whose host path doesn't exist, matching the
	// os.Stat-guarded pattern already used for the hardcoded cache dirs.
	ExtraCacheMounts map[string]string
}

// DefaultIdleTimeout is the idle-activity timeout (Phase 7, "Smart Idle
// Timeout") applied when CommandRequest.IdleTimeout is unset.
const DefaultIdleTimeout = 15 * time.Minute

// LogsContainerDir is the fixed in-container path CommandRequest.LogsHostDir
// is bind-mounted to. mcp-context (see cmd/mcp-context) writes its
// application log and JSON-RPC trace here by default.
const LogsContainerDir = "/var/log/autocode"

// TerminalSize describes a PTY's dimensions in character cells.
type TerminalSize struct {
	Rows uint
	Cols uint
}

type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string

	// UpdatedCredentialFiles holds the post-run content of every path in
	// CommandRequest.CredentialFiles whose content changed during the run
	// (e.g. a CLI silently refreshing an OAuth token on use), keyed the same
	// way as CredentialFiles (target container path -> content). Only
	// populated by runtimes where the credential mount is a live bind (not a
	// copy), so a write inside the container is visible on the host the
	// moment the container stops — nil/empty otherwise.
	UpdatedCredentialFiles map[string]string

	// Killed is true when the runtime itself force-terminated the container
	// (Phase 7: idle timeout or loop detection) rather than the subprocess
	// exiting on its own. Run still returns a normal (nil-error) result
	// with whatever Stdout/Stderr was captured up to the kill — the caller
	// decides how to treat it via KillReason, same as it already does for
	// non-zero ExitCode.
	Killed bool
	// KillReason is "idle_timeout", "loop_detected", or "oom_killed" when
	// Killed is true, "" otherwise.
	KillReason string
}

const (
	KillReasonIdleTimeout  = "idle_timeout"
	KillReasonLoopDetected = "loop_detected"
	// KillReasonOOM marks a container terminated by the kernel/cgroup OOM
	// killer rather than by our own watchForStall watchdog: the process
	// receives SIGKILL externally (exit code 137) with no stallReason set,
	// so without this the run looked like a plain crash and no resume
	// session was ever saved for it (REQ-003) even though OOM interruptions
	// are exactly the kind of failure a resume should recover from.
	KillReasonOOM = "oom_killed"
)

type Runtime interface {
	Run(ctx context.Context, req CommandRequest) (*CommandResult, error)
	RunInteractive(ctx context.Context, req CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error
	Prewarm(ctx context.Context) error
}

type StubRuntime struct{}

func NewStubRuntime() *StubRuntime {
	return &StubRuntime{}
}

func (r *StubRuntime) RunInteractive(ctx context.Context, req CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

func (r *StubRuntime) Prewarm(ctx context.Context) error {
	return nil
}

func (r *StubRuntime) Run(ctx context.Context, req CommandRequest) (*CommandResult, error) {
	ctx, span := otel.Tracer("auto-code-os/sandbox").Start(ctx, "sandbox.stub.run")
	defer span.End()
	if err := validateCommand(req.Command); err != nil {
		return nil, err
	}
	// The stub runs nothing, so treat every request as network-isolated.
	if err := validateExecutionPolicy(req, NetworkModeNone); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &CommandResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("stub sandbox executed: %s", strings.Join(req.Command, " ")),
		Stderr:   "",
	}, nil
}

func mergedEnv(req CommandRequest) map[string]string {
	env := make(map[string]string, len(req.Env)+len(req.SecretEnv))
	maps.Copy(env, req.Env)
	maps.Copy(env, req.SecretEnv)
	return env
}
