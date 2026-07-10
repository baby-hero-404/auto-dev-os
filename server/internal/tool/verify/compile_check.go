package verify

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

var compileErrorRegex = regexp.MustCompile(`^([^:\s]+):(\d+):(?:\d+:)?\s*(.*)$`)

// CompileCheckHook verifies compilation of the package inside the sandbox.
type CompileCheckHook struct {
	Runtime sandbox.Runtime
}

// NewCompileCheckHook creates a new CompileCheckHook.
func NewCompileCheckHook(runtime sandbox.Runtime) *CompileCheckHook {
	return &CompileCheckHook{Runtime: runtime}
}

// Name returns the hook name.
func (h *CompileCheckHook) Name() string { return "compile_check" }

// Run executes a compilation check in the sandbox.
func (h *CompileCheckHook) Run(ctx context.Context, workspace string, files []string) []tool.Diagnostic {
	res, err := h.Runtime.Run(ctx, sandbox.CommandRequest{
		Workspace: workspace,
		Command:   []string{"go", "build", "./..."},
	})
	if err != nil {
		return []tool.Diagnostic{
			{Severity: "error", Message: fmt.Sprintf("compiler check failed: %v", err)},
		}
	}

	if res.ExitCode == 0 {
		return nil
	}

	var diags []tool.Diagnostic
	outputLines := strings.Split(res.Stdout+"\n"+res.Stderr, "\n")
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := compileErrorRegex.FindStringSubmatch(line)
		if len(matches) == 4 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])
			message := matches[3]
			diags = append(diags, tool.Diagnostic{
				Severity: "error",
				File:     file,
				Line:     lineNum,
				Message:  message,
			})
		}
	}

	if len(diags) == 0 {
		diags = append(diags, tool.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("compile check failed with exit code %d but no parsed diagnostics", res.ExitCode),
		})
	}

	return diags
}
