package engine

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

const (
	authPreflightTimeout = 30 * time.Second
	binaryCheckTimeout   = 30 * time.Second
	defaultCLITimeout    = 30 * time.Minute

	// captureFileMarker/captureFileEndMarker delimit a base64-encoded capture
	// block in the subprocess's combined stdout/stderr output. The path
	// requested (relative to ContainerWorkDir) is appended to the start
	// marker so extractCapturedFiles can key CodeStepResult.Files by it.
	captureFileMarker    = "\x00AUTOCODE_CAPTURE_START:"
	captureFileEndMarker = "\x00AUTOCODE_CAPTURE_END\x00"
)

// extractCapturedFiles pulls capture blocks (written by the RunCodeStep
// script) out of combined subprocess output, returning the decoded file
// contents keyed by their requested relative path and the output with those
// blocks removed.
func extractCapturedFiles(combined string) (string, map[string]string) {
	files := make(map[string]string)
	for {
		startIdx := strings.Index(combined, captureFileMarker)
		if startIdx < 0 {
			break
		}
		endIdx := strings.Index(combined[startIdx:], captureFileEndMarker)
		if endIdx < 0 {
			break
		}
		endIdx += startIdx

		block := combined[startIdx+len(captureFileMarker) : endIdx]
		nlIdx := strings.IndexByte(block, '\n')
		var relPath, encoded string
		if nlIdx < 0 {
			relPath = strings.TrimSpace(block)
		} else {
			relPath = strings.TrimSpace(block[:nlIdx])
			encoded = strings.TrimSpace(block[nlIdx+1:])
		}
		if relPath != "" && encoded != "" {
			if decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\n", "")); err == nil {
				files[relPath] = string(decoded)
			}
		}

		combined = combined[:startIdx] + combined[endIdx+len(captureFileEndMarker):]
	}
	if len(files) == 0 {
		return combined, nil
	}
	return combined, files
}

// cliEngine spawns a generic, configurable CLI subprocess inside the
// existing sandbox container. The prompt is written to a file
// (.autocode/prompt.md) rather than passed as an argv value, to avoid
// shell-escaping/length limits; success/failure is judged purely by the
// process exit code and the git diff the caller inspects afterwards.
type cliEngine struct {
	runtime sandbox.Runtime
}

// NewCLIEngine constructs the subprocess-CLI execution engine.
func NewCLIEngine(runtime sandbox.Runtime) ExecutionEngine {
	return &cliEngine{runtime: runtime}
}

func (e *cliEngine) Name() string { return models.ExecutionEngineCLI }

// Preflight checks the configured binary is present in the sandbox and, if
// an auth_check_command is configured, that it succeeds. Both checks run
// with CI=1 set and no stdin attached (the sandbox runtime never opens
// stdin/tty for spawned commands), so an interactive OAuth/login prompt
// cannot hang the check.
func (e *cliEngine) Preflight(ctx context.Context, req CodeStepRequest) (string, error) {
	cfg := req.CLIConfig
	if cfg == nil || strings.TrimSpace(cfg.Command) == "" {
		return "", fmt.Errorf("cli engine: cli_engine_config.command is required")
	}

	checkCmd := fmt.Sprintf("command -v %s >/dev/null 2>&1", paths.QuoteShellArg(cfg.Command))
	res, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      req.Task.ID,
		AgentID:     agentID(req.Agent),
		Workspace:   req.HostWorkspace,
		Command:     []string{"bash", "-lc", checkCmd},
		Env:         map[string]string{"CI": "1"},
		NetworkMode: req.NetworkMode,
		Timeout:     binaryCheckTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("cli engine: preflight failed to run: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("cli engine: command %q not found in sandbox", cfg.Command)
	}

	if strings.TrimSpace(cfg.AuthCheckCommand) == "" {
		if len(cfg.Env) == 0 {
			return "cli engine: no auth_check_command and no env configured — authentication verification is effectively disabled; a stale/expired token may fail mid-run instead of at preflight. Configure auth_check_command or env to enable this check.", nil
		}
		return "", nil
	}

	env := cloneEnv(cfg.Env)
	env["CI"] = "1"
	authRes, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      req.Task.ID,
		AgentID:     agentID(req.Agent),
		Workspace:   req.HostWorkspace,
		Command:     []string{"bash", "-lc", cfg.AuthCheckCommand},
		Env:         env,
		NetworkMode: req.NetworkMode,
		Timeout:     authPreflightTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("cli engine: auth check failed to run: %w", err)
	}
	if authRes.ExitCode != 0 {
		return "", fmt.Errorf("cli engine: auth check command exited %d: %s", authRes.ExitCode, redactSecrets(strings.TrimSpace(authRes.Stderr)))
	}
	return "", nil
}

// RunCodeStep writes the instruction to .autocode/prompt.md inside the
// worktree, spawns the configured CLI with {prompt_file}/{workdir}
// placeholders substituted, and cleans up .autocode/ afterward so the
// prompt file never ends up committed. Success is decided by exit code and
// post-hoc loop detection over the captured output (Runtime.Run is a
// blocking call with no live streaming, so early-kill mid-run is not
// possible with the current sandbox interface).
func (e *cliEngine) RunCodeStep(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error) {
	cfg := req.CLIConfig
	if cfg == nil || strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("cli engine: cli_engine_config.command is required")
	}

	timeout := req.Timeout
	if timeout <= 0 && cfg.TimeoutMinutes > 0 {
		timeout = time.Duration(cfg.TimeoutMinutes) * time.Minute
	}
	if timeout <= 0 {
		timeout = defaultCLITimeout
	}

	autocodeDir := req.ContainerWorkDir + "/.autocode"
	promptFile := autocodeDir + "/prompt.md"
	encodedPrompt := base64.StdEncoding.EncodeToString([]byte(req.Instruction))

	args := make([]string, len(cfg.Args))
	for i, a := range cfg.Args {
		a = strings.ReplaceAll(a, "{prompt_file}", promptFile)
		a = strings.ReplaceAll(a, "{workdir}", req.ContainerWorkDir)
		args[i] = a
	}
	invocation := append([]string{cfg.Command}, args...)
	quotedInvocation := make([]string, len(invocation))
	for i, p := range invocation {
		quotedInvocation[i] = paths.QuoteShellArg(p)
	}

	var captureScript strings.Builder
	for _, rel := range req.CaptureFiles {
		abs := req.ContainerWorkDir + "/" + strings.TrimPrefix(rel, "/")
		fmt.Fprintf(&captureScript,
			" ; echo %s ; if [ -f %s ]; then base64 %s; fi ; echo %s",
			paths.QuoteShellArg(captureFileMarker+rel),
			paths.QuoteShellArg(abs),
			paths.QuoteShellArg(abs),
			paths.QuoteShellArg(captureFileEndMarker),
		)
	}

	script := fmt.Sprintf(
		"cd %s && mkdir -p %s && printf '%%s' '%s' | base64 -d > %s && %s; status=$?%s; rm -rf %s; exit $status",
		paths.QuoteShellArg(req.ContainerWorkDir),
		paths.QuoteShellArg(autocodeDir),
		encodedPrompt,
		paths.QuoteShellArg(promptFile),
		strings.Join(quotedInvocation, " "),
		captureScript.String(),
		paths.QuoteShellArg(autocodeDir),
	)

	env := cloneEnv(cfg.Env)
	env["CI"] = "1"

	result, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      req.Task.ID,
		AgentID:     agentID(req.Agent),
		Workspace:   req.HostWorkspace,
		Command:     []string{"bash", "-lc", script},
		Env:         env,
		NetworkMode: req.NetworkMode,
		Timeout:     timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("cli engine: run failed: %w", err)
	}

	combined := result.Stdout
	if strings.TrimSpace(result.Stderr) != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += result.Stderr
	}
	combined, capturedFiles := extractCapturedFiles(combined)
	killed := detectLoop(combined)

	res := &CodeStepResult{
		Success:    result.ExitCode == 0 && !killed,
		Output:     redactSecrets(combined),
		LoopKilled: killed,
		Files:      capturedFiles,
	}
	switch {
	case killed:
		res.Error = "cli engine: repeated error output detected, killing step as a stuck loop"
	case result.ExitCode != 0:
		res.Error = redactSecrets(fmt.Sprintf("cli exited with status %d", result.ExitCode))
	}
	return res, nil
}

func detectLoop(output string) bool {
	d := newLoopDetector()
	triggered := false
	for line := range strings.SplitSeq(output, "\n") {
		if d.Push(line) {
			triggered = true
		}
	}
	return triggered
}

func cloneEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env)+1)
	maps.Copy(out, env)
	return out
}

func agentID(agent *models.Agent) string {
	if agent == nil {
		return ""
	}
	return agent.ID
}
