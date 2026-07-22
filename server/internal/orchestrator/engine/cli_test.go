package engine

import (
	"context"
	"encoding/base64"
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
}

func (m *mockRuntime) Prewarm(ctx context.Context) error { return nil }

func (m *mockRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	m.calls = append(m.calls, req)
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
		HostWorkspace:    "/host/ws/task-1",
		ContainerWorkDir: "/workspace/backend",
		CLIConfig:        cfg,
	}
}

func TestCLIEngine_Preflight_MissingCommand(t *testing.T) {
	e := NewCLIEngine(&mockRuntime{})
	_, err := e.Preflight(context.Background(), baseReq(nil))
	if err == nil {
		t.Fatal("expected error when cli_engine_config is nil")
	}
}

func TestCLIEngine_Preflight_BinaryNotFound(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1}}}
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(rt)
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

func TestCLIEngine_Preflight_Success(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{
		{ExitCode: 0},
		{ExitCode: 0},
	}}
	e := NewCLIEngine(rt)
	cfg := &models.CLIEngineConfig{Command: "claude", AuthCheckCommand: "claude auth status"}
	if _, err := e.Preflight(context.Background(), baseReq(cfg)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLIEngine_Preflight_WarnsWhenNoAuthConfig(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0}}}
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(rt)
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

func TestCLIEngine_RunCodeStep_MissingCommand(t *testing.T) {
	e := NewCLIEngine(&mockRuntime{})
	_, err := e.RunCodeStep(context.Background(), baseReq(nil))
	if err == nil {
		t.Fatal("expected error when cli_engine_config is nil")
	}
}

func TestCLIEngine_RunCodeStep_RedactsSecrets(t *testing.T) {
	rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 0, Stdout: "token sk-ant-" + strings.Repeat("a", 95)}}}
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(rt)
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
	e := NewCLIEngine(&mockRuntime{})
	if e.Name() != models.ExecutionEngineCLI {
		t.Errorf("Name() = %q, want %q", e.Name(), models.ExecutionEngineCLI)
	}
}
