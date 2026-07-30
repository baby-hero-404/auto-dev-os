package engine

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// ErrConfigInvalid marks a cli engine failure as a permanent configuration
// problem (missing command, unresolvable linked credential, ...) rather than
// a transient one. Callers should not burn retry attempts on it — the same
// misconfiguration will fail identically every time until a human fixes it.
var ErrConfigInvalid = errors.New("cli engine: invalid configuration")

const (
	authPreflightTimeout = 30 * time.Second
	binaryCheckTimeout   = 30 * time.Second
	defaultCLITimeout    = 30 * time.Minute

	// captureFileMarker/captureFileEndMarker delimit a base64-encoded capture
	// block in the subprocess's combined stdout/stderr output. The path
	// requested (relative to ContainerWorkDir) is appended to the start
	// marker so extractCapturedFiles can key CodeStepResult.Files by it.
	captureFileMarker    = "===AUTOCODE_CAPTURE_START:"
	captureFileEndMarker = "===AUTOCODE_CAPTURE_END==="
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

// CredentialGetter resolves an org-scoped saved credential to its decrypted
// payload, matching service.CredentialPoolService.GetDecryptedCredential.
// The payload maps a relative path (as captured during interactive CLI
// login, e.g. ".claude.json" or ".config/codex/auth.json") to file content.
type CredentialGetter interface {
	GetDecryptedCredential(ctx context.Context, orgID, id string) (provider string, payload map[string]string, err error)

	// UpdateCredentialPayload re-encrypts and persists an updated credential
	// payload — best-effort write-back after a CLI run refreshes a token
	// on-disk (e.g. OAuth rotation), so the next run doesn't replay a stale
	// one. Callers must not fail the run itself if this errors.
	UpdateCredentialPayload(ctx context.Context, orgID, id string, payload map[string]string) error
}

// cliEngine spawns a generic, configurable CLI subprocess inside the
// existing sandbox container. The prompt is written to a file
// (.autocode/prompt.md) rather than passed as an argv value, to avoid
// shell-escaping/length limits; success/failure is judged purely by the
// process exit code and the git diff the caller inspects afterwards.
type cliEngine struct {
	runtime     sandbox.Runtime
	credentials CredentialGetter
}

// NewCLIEngine constructs the subprocess-CLI execution engine. credentials
// may be nil, in which case CLIConfig.CredentialID is ignored (the run
// proceeds unauthenticated, same as before credential linking existed).
func NewCLIEngine(runtime sandbox.Runtime, credentials CredentialGetter) ExecutionEngine {
	return &cliEngine{runtime: runtime, credentials: credentials}
}

// resolveCredentialFiles fetches the linked saved credential (if any) and
// maps its relative-path payload onto absolute container paths under
// /root, matching the layout the interactive CLI auth flow captured it in
// (see cli_auth.go's Terminal handler, which runs with HOME=/workspace
// during capture) and the container's runtime HOME of /root.
func (e *cliEngine) resolveCredentialFiles(ctx context.Context, req CodeStepRequest) (map[string]string, map[string]string, error) {
	cfg := req.CLIConfig
	if cfg == nil || strings.TrimSpace(cfg.CredentialID) == "" {
		return nil, nil, nil
	}
	if e.credentials == nil {
		return nil, nil, fmt.Errorf("cli engine: cli_engine_config.credential_id is set but no credential service is configured: %w", ErrConfigInvalid)
	}
	_, payload, err := e.credentials.GetDecryptedCredential(ctx, req.OrgID, cfg.CredentialID)
	if err != nil {
		return nil, nil, fmt.Errorf("cli engine: failed to load linked credential: %w", err)
	}
	files := make(map[string]string, len(payload))
	for relPath, content := range payload {
		clean := filepath.Clean("/" + relPath)
		if clean == "/" || strings.Contains(clean, "..") {
			continue
		}
		files[sandbox.SandboxHomeDir+clean] = content
	}
	return files, payload, nil
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
		return "", fmt.Errorf("cli engine: cli_engine_config.command is required: %w", ErrConfigInvalid)
	}

	credentialFiles, _, err := e.resolveCredentialFiles(ctx, req)
	if err != nil {
		return "", err
	}

	checkCmd := fmt.Sprintf("command -v %s >/dev/null 2>&1", paths.QuoteShellArg(cfg.Command))
	res, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:          req.Task.ID,
		AgentID:         agentID(req.Agent),
		Workspace:       req.HostWorkspace,
		Command:         []string{"bash", "-lc", checkCmd},
		Env:             map[string]string{"CI": "1", "HOME": sandbox.SandboxHomeDir},
		NetworkMode:     req.NetworkMode,
		Timeout:         binaryCheckTimeout,
		CredentialFiles: credentialFiles,
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
	if _, ok := env["HOME"]; !ok {
		env["HOME"] = sandbox.SandboxHomeDir
	}
	authRes, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:          req.Task.ID,
		AgentID:         agentID(req.Agent),
		Workspace:       req.HostWorkspace,
		Command:         []string{"bash", "-lc", cfg.AuthCheckCommand},
		Env:             env,
		NetworkMode:     req.NetworkMode,
		Timeout:         authPreflightTimeout,
		CredentialFiles: credentialFiles,
	})
	if err != nil {
		return "", fmt.Errorf("cli engine: auth check failed to run: %w", err)
	}
	if authRes.ExitCode != 0 {
		return "", fmt.Errorf("cli engine: auth check command exited %d: %s", authRes.ExitCode, redactSecrets(strings.TrimSpace(authRes.Stderr)))
	}
	// Some CLIs' auth-status commands (e.g. `claude auth status`, `codex
	// login status`) are informational and exit 0 regardless of login
	// state, reporting it via output content instead (REQ-003) — exit code
	// alone would let a genuinely logged-out credential pass Preflight, the
	// exact gap that let task cfeacf66-... burn 3 retries on "Not logged in".
	authOutput := authRes.Stdout
	if strings.TrimSpace(authRes.Stderr) != "" {
		if authOutput != "" {
			authOutput += "\n"
		}
		authOutput += authRes.Stderr
	}
	if detectAuthInvalid(cfg.ProfileRef, authOutput) {
		return "", fmt.Errorf("cli engine: auth check command reports not authenticated: %s", redactSecrets(strings.TrimSpace(authOutput)))
	}
	return "", nil
}

// hostPathForContainerPath maps a path inside the sandbox container back to
// its host location, given that hostWorkspace is bind-mounted at the
// container workspace root ("/workspace" — see sandbox.DockerRuntime and
// pkg/paths helpers, which share this convention).
func hostPathForContainerPath(hostWorkspace, containerPath string) (string, error) {
	const containerRoot = "/workspace"
	if hostWorkspace == "" {
		return "", fmt.Errorf("host workspace is required to map container path %q", containerPath)
	}
	if containerPath == containerRoot {
		return hostWorkspace, nil
	}
	rel, ok := strings.CutPrefix(containerPath, containerRoot+"/")
	if !ok {
		return "", fmt.Errorf("container path %q is outside the workspace mount %q", containerPath, containerRoot)
	}
	return filepath.Join(hostWorkspace, rel), nil
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
		return nil, fmt.Errorf("cli engine: cli_engine_config.command is required: %w", ErrConfigInvalid)
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

	// Write the prompt to the host side of the bind mount before spawning,
	// instead of inlining it (base64) into the bash script: a single argv
	// string is capped at MAX_ARG_STRLEN (128KB on Linux), so large prompts
	// inlined into the script would make execve fail with E2BIG.
	hostAutocodeDir, err := hostPathForContainerPath(req.HostWorkspace, autocodeDir)
	if err != nil {
		return nil, fmt.Errorf("cli engine: %w", err)
	}
	if err := os.MkdirAll(hostAutocodeDir, 0o755); err != nil {
		return nil, fmt.Errorf("cli engine: create prompt dir: %w", err)
	}

	contextRoot := filepath.Join(hostAutocodeDir, "context")
	for relPath, content := range req.ContextFiles {
		target := filepath.Join(contextRoot, filepath.FromSlash(relPath))
		// Defense in depth: relPath is derived from platform-scored data
		// (e.g. skill names sourced from third-party git repos) that isn't
		// guaranteed to be traversal-free upstream. Refuse anything that
		// would resolve outside contextRoot rather than trusting the caller.
		rel, err := filepath.Rel(contextRoot, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("cli engine: context file path %q escapes context dir", relPath)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, fmt.Errorf("cli engine: create context dir for %s: %w", relPath, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("cli engine: write context file %s: %w", relPath, err)
		}
	}
	// Backup cleanup in case the sandbox never runs the in-script rm -rf
	// (e.g. container create fails); idempotent when the script already
	// removed the container-side copy.
	defer os.RemoveAll(hostAutocodeDir)
	if err := os.WriteFile(filepath.Join(hostAutocodeDir, "prompt.md"), []byte(req.Instruction), 0o644); err != nil {
		return nil, fmt.Errorf("cli engine: write prompt file: %w", err)
	}

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
		"cd %s && %s; status=$?%s; rm -rf %s; exit $status",
		paths.QuoteShellArg(req.ContainerWorkDir),
		strings.Join(quotedInvocation, " "),
		captureScript.String(),
		paths.QuoteShellArg(autocodeDir),
	)

	env := cloneEnv(cfg.Env)
	env["CI"] = "1"
	if _, ok := env["HOME"]; !ok {
		env["HOME"] = sandbox.SandboxHomeDir
	}
	if len(req.ContextFiles) > 0 {
		env["AUTOCODE_CONTEXT_DIR"] = autocodeDir + "/context"
	}

	credentialFiles, _, err := e.resolveCredentialFiles(ctx, req)
	if err != nil {
		return nil, err
	}

	result, err := e.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:          req.Task.ID,
		AgentID:         agentID(req.Agent),
		Workspace:       req.HostWorkspace,
		Command:         []string{"bash", "-lc", script},
		Env:             env,
		NetworkMode:     req.NetworkMode,
		Timeout:         timeout,
		CredentialFiles: credentialFiles,
	})
	if err != nil {
		return nil, fmt.Errorf("cli engine: run failed: %w", err)
	}

	if cfg.CredentialID != "" && len(result.UpdatedCredentialFiles) > 0 && e.credentials != nil {
		// Merge onto the credential's current DB state, fetched fresh right
		// here rather than reusing the pre-run snapshot from
		// resolveCredentialFiles: another step/task sharing this credential
		// may have refreshed a different file in between, and merging onto a
		// stale base would silently revert that update. Only the specific
		// relPaths the run actually changed are touched — never a full
		// overwrite, so a file that got added or removed in the DB by
		// someone else concurrently is left alone.
		if _, fresh, err := e.credentials.GetDecryptedCredential(ctx, req.OrgID, cfg.CredentialID); err == nil {
			merged := make(map[string]string, len(fresh))
			maps.Copy(merged, fresh)
			for targetPath, newContent := range result.UpdatedCredentialFiles {
				relPath, ok := strings.CutPrefix(targetPath, sandbox.SandboxHomeDir+"/")
				if !ok {
					continue
				}
				merged[relPath] = newContent
			}
			// Best-effort: a refreshed-token write-back failing must never
			// fail the step itself (the run already succeeded/failed on its
			// own terms) — only the *next* run would see a stale token.
			_ = e.credentials.UpdateCredentialPayload(ctx, req.OrgID, cfg.CredentialID, merged)
		}
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
	authInvalid := detectAuthInvalid(cfg.ProfileRef, combined)
	// Auth-invalid takes priority over quota: both are runtime failure
	// signatures but auth-invalid is permanent (retrying never helps) while
	// quota is transient (cools down and retries later) — reporting both on
	// the same output would let the quota cooldown path mask the permanent
	// one.
	quotaExceeded := !authInvalid && detectQuotaExceeded(cfg.ProfileRef, combined, result.ExitCode)
	// Checked regardless of exit code: some CLIs print a question and then
	// exit 0 ("can't prompt non-interactively"), which would otherwise be
	// read as a silent success. Skipped when a more specific outcome
	// (killed loop, auth failure) already explains why the run stopped.
	awaitingInput := !killed && !authInvalid && detectAwaitingInput(lastNonEmptyLine(combined))

	credID := ""
	if cfg != nil {
		credID = cfg.CredentialID
	}
	res := &CodeStepResult{
		Success:                 result.ExitCode == 0 && !killed && !awaitingInput && !authInvalid,
		Output:                  redactSecrets(combined),
		LoopKilled:              killed,
		QuotaExceeded:           quotaExceeded,
		AuthInvalid:             authInvalid,
		AwaitingInput:           awaitingInput,
		Files:                   capturedFiles,
		ExitCode:                result.ExitCode,
		Command:                 redactSecrets(strings.Join(quotedInvocation, " ")),
		CredentialID:            credID,
		CredentialFilesResolved: len(credentialFiles),
	}
	switch {
	case killed:
		res.Error = "cli engine: repeated error output detected, killing step as a stuck loop"
	case authInvalid:
		res.Error = redactSecrets(fmt.Sprintf("cli engine: credential not authenticated (permanent, will not retry): %s", lastNonEmptyLine(combined)))
	case awaitingInput:
		res.Error = redactSecrets(fmt.Sprintf("cli engine: process appears to be waiting for input: %s", lastNonEmptyLine(combined)))
	case result.ExitCode != 0:
		res.Error = redactSecrets(fmt.Sprintf("cli exited with status %d", result.ExitCode))
	}
	return res, nil
}

// lastNonEmptyLine returns the last non-blank line of s, trimmed, or "" if
// every line is blank.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
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
