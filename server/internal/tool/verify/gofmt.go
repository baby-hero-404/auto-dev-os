package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// GofmtHook formats changed Go files inside the sandbox.
type GofmtHook struct {
	Runtime sandbox.Runtime
}

// NewGofmtHook creates a new GofmtHook.
func NewGofmtHook(runtime sandbox.Runtime) *GofmtHook {
	return &GofmtHook{Runtime: runtime}
}

// Name returns the hook name.
func (h *GofmtHook) Name() string { return "gofmt" }

// Run executes gofmt -w on the changed Go files.
func (h *GofmtHook) Run(ctx context.Context, workspace string, files []string) []tool.Diagnostic {
	var diags []tool.Diagnostic
	var goFiles []string
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			goFiles = append(goFiles, f)
		}
	}
	if len(goFiles) == 0 {
		return nil
	}

	for _, f := range goFiles {
		res, err := h.Runtime.Run(ctx, sandbox.CommandRequest{
			Workspace: workspace,
			Command:   []string{"gofmt", "-w", f},
		})
		if err != nil {
			diags = append(diags, tool.Diagnostic{
				Severity: "error",
				File:     f,
				Message:  fmt.Sprintf("gofmt execution failed: %v", err),
			})
			continue
		}
		if res.ExitCode != 0 {
			diags = append(diags, tool.Diagnostic{
				Severity: "error",
				File:     f,
				Message:  fmt.Sprintf("gofmt failed with exit status %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr)),
			})
		}
	}
	return diags
}
