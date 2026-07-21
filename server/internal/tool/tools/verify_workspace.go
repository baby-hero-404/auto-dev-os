package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// VerifyWorkspaceTool runs build, lint, and test sequentially in one LLM iteration.
type VerifyWorkspaceTool struct {
	Runtime sandbox.Runtime
	build   *RunBuildTool
	lint    *RunLintTool
	tests   *RunTestsTool
}

func NewVerifyWorkspaceTool(runtime sandbox.Runtime) *VerifyWorkspaceTool {
	return &VerifyWorkspaceTool{
		Runtime: runtime,
		build:   NewRunBuildTool(runtime),
		lint:    NewRunLintTool(runtime),
		tests:   NewRunTestsTool(runtime),
	}
}

func (t *VerifyWorkspaceTool) Name() string { return "verify_workspace" }

func (t *VerifyWorkspaceTool) Category() tool.Category { return tool.CategoryBuild }

func (t *VerifyWorkspaceTool) Capabilities() []tool.Capability {
	return []tool.Capability{tool.CapBuild}
}

func (t *VerifyWorkspaceTool) Description() string {
	return "Run build, lint, and tests sequentially in one tool call. Use this to verify the workspace efficiently instead of running each check individually."
}

func (t *VerifyWorkspaceTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"build_command": {"type": "string", "description": "Optional build command (e.g. go build ./...)"},
			"lint_command": {"type": "string", "description": "Optional lint command (e.g. golangci-lint run)"},
			"test_command": {"type": "string", "description": "Optional test command (e.g. go test ./...)"}
		}
	}`)
}

func (t *VerifyWorkspaceTool) Execute(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.Runtime == nil {
		return tool.Result{Success: false, Message: "sandbox runtime is not configured"}, nil
	}

	buildCmd, _ := call.Input["build_command"].(string)
	lintCmd, _ := call.Input["lint_command"].(string)
	testCmd, _ := call.Input["test_command"].(string)

	var diagnostics []tool.Diagnostic
	var output string
	success := true

	// 1. Build
	buildCall := call
	if buildCmd != "" {
		buildCall.Input = map[string]any{"command": buildCmd}
	} else {
		buildCall.Input = map[string]any{}
	}
	buildRes, err := t.build.Execute(ctx, buildCall)
	if err != nil {
		return tool.Result{}, fmt.Errorf("build phase failed: %w", err)
	}
	output += "=== BUILD PHASE ===\n" + buildRes.Output + "\n"
	diagnostics = append(diagnostics, buildRes.Diagnostics...)
	if !buildRes.Success {
		success = false
	}

	// 2. Lint (only if build succeeded or if there are no build errors)
	if success {
		lintCall := call
		if lintCmd != "" {
			lintCall.Input = map[string]any{"command": lintCmd}
		} else {
			lintCall.Input = map[string]any{}
		}
		lintRes, err := t.lint.Execute(ctx, lintCall)
		if err != nil {
			return tool.Result{}, fmt.Errorf("lint phase failed: %w", err)
		}
		output += "=== LINT PHASE ===\n" + lintRes.Output + "\n"
		diagnostics = append(diagnostics, lintRes.Diagnostics...)
		if !lintRes.Success {
			success = false
		}
	} else {
		output += "=== LINT PHASE ===\nSkipped due to build failure\n"
	}

	// 3. Test (only if build succeeded)
	if success {
		testCall := call
		if testCmd != "" {
			testCall.Input = map[string]any{"command": testCmd}
		} else {
			testCall.Input = map[string]any{}
		}
		testRes, err := t.tests.Execute(ctx, testCall)
		if err != nil {
			return tool.Result{}, fmt.Errorf("test phase failed: %w", err)
		}
		output += "=== TEST PHASE ===\n" + testRes.Output + "\n"
		diagnostics = append(diagnostics, testRes.Diagnostics...)
		if !testRes.Success {
			success = false
		}
	} else {
		output += "=== TEST PHASE ===\nSkipped due to prior failure\n"
	}

	return tool.Result{
		Success:     success,
		Output:      output,
		Diagnostics: diagnostics,
	}, nil
}
