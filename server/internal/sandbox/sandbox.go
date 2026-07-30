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
}

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
}

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
