package engine

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockRuntime struct {
	calls   []sandbox.CommandRequest
	results []*sandbox.CommandResult
	errs    []error
	i       int
	onRun   func(req sandbox.CommandRequest)
}

func (m *mockRuntime) Prewarm(ctx context.Context) error { return nil }

func (m *mockRuntime) RunInteractive(ctx context.Context, req sandbox.CommandRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	return nil
}

func (m *mockRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.calls = append(m.calls, req)
	if m.onRun != nil {
		m.onRun(req)
	}
	idx := m.i
	m.i++
	if idx < len(m.errs) && m.errs[idx] != nil {
		return nil, m.errs[idx]
	}
	if idx < len(m.results) {
		return m.results[idx], nil
	}
	return &sandbox.CommandResult{ExitCode: 0}, nil
}

func baseReq(cfg *models.CLIEngineConfig) CodeStepRequest {
	return CodeStepRequest{
		Task:             &models.Task{ID: "task-1"},
		Agent:            &models.Agent{ID: "agent-1"},
		Instruction:      "implement the feature",
		HostWorkspace:    hostWorkspaceForTest,
		ContainerWorkDir: "/workspace/backend",
		CLIConfig:        cfg,
	}
}

// hostWorkspaceForTest is a real writable directory: RunCodeStep now writes
// the prompt file to the host side of the workspace bind mount before
// spawning, so the host workspace must exist.
var hostWorkspaceForTest = func() string {
	dir, err := os.MkdirTemp("", "cli-engine-test-ws-")
	if err != nil {
		panic(err)
	}
	return dir
}()

func TestCLIEngine_Preflight_MissingCommand(t *testing.T) {
	e := NewCLIEngine(&mockRuntime{}, nil)
	_, err := e.Preflight(context.Background(), baseReq(nil))
	if err == nil {
		t.Fatal("expected error when cli_engine_config is nil")
	}
}

func TestCLIEngine_Preflight_BinaryNotFound(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude"}
	_, err := e.Preflight(context.Background(), baseReq(cfg))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

func TestCLIEngine_Preflight_AuthCheckFails(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{
		{ExitCode: 0},                          // binary check ok
		{ExitCode: 1, Stderr: "not logged in"}, // auth check fails
	}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", AuthCheckCommand: "claude auth status"}
	_, err := e.Preflight(context.Background(), baseReq(cfg))
	if err == nil || !strings.Contains(err.Error(), "auth check") {
		t.Fatalf("expected auth check error, got %v", err)
	}
	// Ensure CI=1 was set and no auth prompt could block on stdin.
	if rt.calls[1].Env["CI"] != "1" {
		t.Errorf("expected CI=1 to be set on auth check invocation")
	}
}

func TestCLIEngine_Preflight_AuthCheckExitsZeroButReportsNotLoggedIn(t *testing.T) {
	// claude auth status / codex login status are informational commands
	// that exit 0 regardless of login state — the failure signal is in the
	// output content, not the exit code (REQ-003). Preflight must catch
	// this via detectAuthInvalid, not exit-code alone.
	rt := &mockRuntime{results: []*sandbox.CommandResult{
		{ExitCode: 0},                                       // binary check ok
		{ExitCode: 0, Stdout: "{\n  \"loggedIn\": false\n}"}, // auth check "succeeds" but reports logged out
	}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", AuthCheckCommand: "claude auth status", ProfileRef: "claude_code"}
	_, err := e.Preflight(context.Background(), baseReq(cfg))
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("expected 'not authenticated' error despite exit code 0, got %v", err)
	}
}

func TestCLIEngine_Preflight_Success(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{
		{ExitCode: 0},
		{ExitCode: 0},
	}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", AuthCheckCommand: "claude auth status"}
	if _, err := e.Preflight(context.Background(), baseReq(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLIEngine_Preflight_WarnsWhenNoAuthConfig(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude"}
	warning, err := e.Preflight(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warning == "" {
		t.Fatal("expected a non-empty warning when auth_check_command and env are both empty")
	}
}

func TestCLIEngine_Preflight_NoWarningWhenEnvConfigured(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", Env: map[string]string{"ANTHROPIC_API_KEY": "sk-x"}}
	warning, err := e.Preflight(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warning != "" {
		t.Errorf("expected no warning when env is configured, got: %s", warning)
	}
}

func TestCLIEngine_RunCodeStep_Success(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "all good"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", Args: []string{"-p", "--file", "{prompt_file}"}}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got %+v", res)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("expected exactly 1 sandbox call, got %d", len(rt.calls))
	}
	script := rt.calls[0].Command[2]
	if !strings.Contains(script, "/workspace/backend/.autocode/prompt.md") {
		t.Errorf("expected script to reference the resolved prompt file path, got: %s", script)
	}
	if !strings.Contains(script, "rm -rf") {
		t.Errorf("expected script to clean up .autocode dir, got: %s", script)
	}
	if rt.calls[0].Env["CI"] != "1" {
		t.Errorf("expected CI=1 on the real spawn")
	}
}

func TestCLIEngine_RunCodeStep_NonZeroExit(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 2, Stderr: "boom"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Errorf("expected failure on non-zero exit code")
	}
	if res.Error == "" {
		t.Errorf("expected an error message to be set")
	}
}

func TestCLIEngine_RunCodeStep_LoopKill(t *testing.T) {
	var lines []string
	for i := 0; i < loopKillThreshold; i++ {
		lines = append(lines, "Error: connection refused")
	}
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: strings.Join(lines, "\n")}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.LoopKilled {
		t.Errorf("expected LoopKilled to be true")
	}
	if res.Success {
		t.Errorf("expected Success to be false when loop-killed even with exit code 0")
	}
}

func TestCLIEngine_RunCodeStep_AuthInvalid(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1, Stdout: "Not logged in · Please run /login\n"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.AuthInvalid {
		t.Errorf("expected AuthInvalid=true, got %+v", res)
	}
	if res.Success {
		t.Errorf("expected Success=false")
	}
}

// TestCLIEngine_RunCodeStep_AuthInvalid_ExitZeroStillMarkedFailed guards a
// fixed bug: Success previously didn't subtract AuthInvalid, so a run whose
// combined output incidentally matched an auth-invalid pattern (e.g. a task
// legitimately working on auth/OAuth code) but exited 0 was reported as
// Success=true while also carrying AuthInvalid=true — the step was accepted
// as complete while its credential got silently disabled via
// MarkNeedsReauth. Success must always be false when AuthInvalid is true,
// mirroring how LoopKilled/AwaitingInput are already subtracted.
func TestCLIEngine_RunCodeStep_AuthInvalid_ExitZeroStillMarkedFailed(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "...token revoked test case passed\nDone.\n"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.AuthInvalid {
		t.Fatalf("expected AuthInvalid=true for this fixture, got %+v", res)
	}
	if res.Success {
		t.Errorf("expected Success=false when AuthInvalid=true, even with ExitCode=0, got %+v", res)
	}
}

func TestCLIEngine_RunCodeStep_AuthInvalid_TakesPriorityOverQuota(t *testing.T) {
	// A message that could plausibly match both rule sets in some CLI
	// output shapes — auth-invalid must win so the caller doesn't cool down
	// a credential that will never work regardless of cooldown.
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1, Stdout: "Not logged in · Please run /login\n"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.QuotaExceeded {
		t.Errorf("expected QuotaExceeded=false when AuthInvalid is true, got %+v", res)
	}
}

func TestCLIEngine_RunCodeStep_AwaitingInput(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1, Stdout: "Analyzing repo...\nProceed with deletion? (y/n)"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.AwaitingInput {
		t.Errorf("expected AwaitingInput=true, got %+v", res)
	}
	if res.Success {
		t.Errorf("expected Success=false when awaiting input")
	}
}

func TestCLIEngine_RunCodeStep_AwaitingInput_ExitZeroStillDetected(t *testing.T) {
	// Some CLIs print a question and then exit 0 ("can't prompt
	// non-interactively") — must not be read as a silent success.
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "Do you want to overwrite config.yaml?"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.AwaitingInput || res.Success {
		t.Errorf("expected AwaitingInput=true, Success=false, got %+v", res)
	}
}

func TestCLIEngine_RunCodeStep_MissingCommand(t *testing.T) {
	e := NewCLIEngine(&mockRuntime{}, nil)
	_, err := e.RunCodeStep(context.Background(), baseReq(nil))
	if err == nil {
		t.Fatal("expected error when cli_engine_config is nil")
	}
}

func TestCLIEngine_RunCodeStep_RedactsSecrets(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "token sk-ant-" + strings.Repeat("a", 95)}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude"}
	res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(res.Output, "sk-ant-") {
		t.Errorf("expected secret to be redacted from output, got: %s", res.Output)
	}
}

func TestCLIEngine_RunCodeStep_CaptureFiles_ScriptWiring(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "done"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude"}
	req := baseReq(cfg)
	req.CaptureFiles = []string{".autocode/analysis.md"}
	if _, err := e.RunCodeStep(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	script := rt.calls[0].Command[2]
	if !strings.Contains(script, "/workspace/backend/.autocode/analysis.md") {
		t.Errorf("expected script to reference the capture file path, got: %s", script)
	}
	// The capture block must run before the .autocode cleanup, otherwise the
	// file would already be gone by the time it's read.
	captureIdx := strings.Index(script, "AUTOCODE_CAPTURE_START")
	cleanupIdx := strings.Index(script, "rm -rf")
	if captureIdx < 0 || cleanupIdx < 0 || captureIdx > cleanupIdx {
		t.Errorf("expected capture block before cleanup, script: %s", script)
	}
}

func TestCLIEngine_RunCodeStep_LargePromptNotInlinedInScript(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "done"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", Args: []string{"--prompt-file", "{prompt_file}"}}
	req := baseReq(cfg)
	// Well past MAX_ARG_STRLEN (128KB): inlining this into the bash script
	// would make execve fail with E2BIG.
	req.Instruction = strings.Repeat("x", 300*1024)

	if _, err := e.RunCodeStep(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	script := rt.calls[0].Command[2]
	if len(script) > 64*1024 {
		t.Errorf("script grew with prompt size (%d bytes) — prompt must not be inlined", len(script))
	}
	if strings.Contains(script, "xxxxxxxxxx") {
		t.Error("expected prompt content to be absent from the script")
	}
	if !strings.Contains(script, "/workspace/backend/.autocode/prompt.md") {
		t.Errorf("expected script to reference the prompt file path, got: %s", script)
	}
}

func TestCLIEngine_RunCodeStep_WritesPromptFileOnHost(t *testing.T) {
	promptSeen := make(chan string, 1)
	rt := &mockRuntime{onRun: func(req sandbox.CommandRequest) {
		// Read the host-side prompt file while the "subprocess" is running,
		// before RunCodeStep's deferred cleanup removes it.
		data, err := os.ReadFile(filepath.Join(hostWorkspaceForTest, "backend", ".autocode", "prompt.md"))
		if err != nil {
			promptSeen <- "READ_ERROR: " + err.Error()
			return
		}
		promptSeen <- string(data)
	}}
	e := NewCLIEngine(rt, nil)
	req := baseReq(&models.CLIEngineConfig{Command: "claude"})
	req.Instruction = "implement the feature"

	if _, err := e.RunCodeStep(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := <-promptSeen; got != "implement the feature" {
		t.Errorf("expected prompt file to contain the instruction, got: %q", got)
	}
}

func TestCLIEngine_RunCodeStep_WritesContextFilesAndEnv(t *testing.T) {
	seen := make(chan struct {
		content string
		env     string
	}, 1)
	rt := &mockRuntime{onRun: func(req sandbox.CommandRequest) {
		data, err := os.ReadFile(filepath.Join(hostWorkspaceForTest, "backend", ".autocode", "context", "relevant", "skills", "foo.md"))
		content := string(data)
		if err != nil {
			content = "READ_ERROR: " + err.Error()
		}
		seen <- struct {
			content string
			env     string
		}{content, req.Env["AUTOCODE_CONTEXT_DIR"]}
	}}
	e := NewCLIEngine(rt, nil)
	req := baseReq(&models.CLIEngineConfig{Command: "claude"})
	req.ContextFiles = map[string]string{"relevant/skills/foo.md": "skill body"}

	if _, err := e.RunCodeStep(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := <-seen
	if got.content != "skill body" {
		t.Errorf("expected context file to contain %q, got %q", "skill body", got.content)
	}
	if got.env != "/workspace/backend/.autocode/context" {
		t.Errorf("expected AUTOCODE_CONTEXT_DIR to be set, got %q", got.env)
	}
}

func TestCLIEngine_RunCodeStep_NoContextFilesMeansNoEnvVar(t *testing.T) {
	seen := make(chan string, 1)
	rt := &mockRuntime{onRun: func(req sandbox.CommandRequest) {
		seen <- req.Env["AUTOCODE_CONTEXT_DIR"]
	}}
	e := NewCLIEngine(rt, nil)
	req := baseReq(&models.CLIEngineConfig{Command: "claude"})

	if _, err := e.RunCodeStep(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := <-seen; got != "" {
		t.Errorf("expected no AUTOCODE_CONTEXT_DIR when ContextFiles is empty, got %q", got)
	}
}

func TestCLIEngine_RunCodeStep_RejectsContextFilePathEscapingContextDir(t *testing.T) {
	e := NewCLIEngine(&mockRuntime{}, nil)
	req := baseReq(&models.CLIEngineConfig{Command: "claude"})
	req.ContextFiles = map[string]string{
		"../../../../etc/cron.d/evil": "malicious",
	}

	_, err := e.RunCodeStep(context.Background(), req)
	if err == nil {
		t.Fatal("expected an error for a context file path that escapes the context dir, got nil")
	}
	if !strings.Contains(err.Error(), "escapes context dir") {
		t.Errorf("expected escape-detection error, got: %v", err)
	}

	escapedPath := filepath.Join(hostWorkspaceForTest, "..", "..", "etc", "cron.d", "evil")
	if _, statErr := os.Stat(escapedPath); statErr == nil {
		t.Fatal("context file was written outside the workspace despite the traversal guard")
	}
}

func TestExtractCapturedFiles(t *testing.T) {
	content := "hello world"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	combined := "some cli output\n" +
		captureFileMarker + ".autocode/analysis.md\n" + encoded + "\n" + captureFileEndMarker +
		"\nmore output"

	cleaned, files := extractCapturedFiles(combined)

	if files[".autocode/analysis.md"] != content {
		t.Fatalf("expected captured content %q, got %q", content, files[".autocode/analysis.md"])
	}
	if strings.Contains(cleaned, "AUTOCODE_CAPTURE") {
		t.Errorf("expected capture markers stripped from output, got: %s", cleaned)
	}
	if !strings.Contains(cleaned, "some cli output") || !strings.Contains(cleaned, "more output") {
		t.Errorf("expected surrounding output preserved, got: %s", cleaned)
	}
}

func TestExtractCapturedFiles_NoCaptures(t *testing.T) {
	cleaned, files := extractCapturedFiles("plain output, nothing to capture")
	if files != nil {
		t.Errorf("expected nil files map when no captures present, got: %v", files)
	}
	if cleaned != "plain output, nothing to capture" {
		t.Errorf("expected output unchanged, got: %s", cleaned)
	}
}

func TestCLIEngine_Name(t *testing.T) {
	e := NewCLIEngine(&mockRuntime{}, nil)
	if e.Name() != models.ExecutionEngineCLI {
		t.Errorf("Name() = %q, want %q", e.Name(), models.ExecutionEngineCLI)
	}
}

func TestCLIEngine_RunCodeStep_SetsSessionMounts(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "done"}}}
	e := NewCLIEngine(rt, nil)
	cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
	req := baseReq(cfg)

	if _, err := e.RunCodeStep(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rt.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(rt.calls))
	}

	expectedHostPath := filepath.Join(req.HostWorkspace, "session", "claude_code", ".claude")
	expectedContainerPath := filepath.Join(sandbox.SandboxHomeDir, ".claude")
	
	if mountPath, ok := rt.calls[0].SessionMounts[expectedContainerPath]; !ok || mountPath != expectedHostPath {
		t.Errorf("expected SessionMounts[%q] = %q, got map: %v", expectedContainerPath, expectedHostPath, rt.calls[0].SessionMounts)
	}
}

func TestCLIEngine_RunCodeStep_ResumeFlags(t *testing.T) {
	tests := []struct {
		profileRef   string
		sessionID    string
		expectedArgs []string
	}{
		{"claude_code", "claude-123", []string{"--resume", "claude-123"}},
		{"antigravity", "agy-456", []string{"--conversation", "agy-456"}},
		{"openai_codex", "codex-789", []string{"resume", "--last"}},
		{"unknown_cli", "unknown-123", nil}, // should not append any resume flags
	}

	for _, tc := range tests {
		t.Run(tc.profileRef, func(t *testing.T) {
			rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0}}}
			e := NewCLIEngine(rt, nil)
			cfg := &models.CLIEngineConfig{Command: "testcmd", ProfileRef: tc.profileRef}
			
			req := baseReq(cfg)
			req.ResumeSessionID = tc.sessionID
			
			_, err := e.RunCodeStep(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if len(rt.calls) == 0 {
				t.Fatal("expected sandbox request")
			}
			
			cmdLine := rt.calls[0].Command[2] // bash -lc '<script>'
			
			for _, exp := range tc.expectedArgs {
				if !strings.Contains(cmdLine, exp) {
					t.Errorf("expected script to contain %q, but got: %s", exp, cmdLine)
				}
			}
		})
	}
}
