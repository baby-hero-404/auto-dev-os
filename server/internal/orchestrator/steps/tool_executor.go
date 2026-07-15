package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

// NewRegistryToolExecutor builds a llmrunner.ToolExecutor backed by a tool.Registry, mirroring
// AnalyzeStep.executeAnalyzeTool's dispatch so review/coding steps can drive the same native
// tool-calling loop (Issue 1+2). workspace must be the physical directory the tool calls should
// operate on (e.g. the role-specific worktree for coding steps).
func NewRegistryToolExecutor(registry *tool.Registry, workspace, taskID, agentID, agentRole string) llmrunner.ToolExecutor {
	return func(ctx context.Context, name, argumentsJSON string) (string, error) {
		if registry == nil {
			return "Error: tool registry not configured", nil
		}

		var args map[string]any
		if strings.TrimSpace(argumentsJSON) != "" {
			if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
				return fmt.Sprintf("Error: invalid tool arguments JSON: %v", err), nil
			}
		}
		if args == nil {
			args = map[string]any{}
		}

		call := tool.Call{
			Input:     args,
			Workspace: workspace,
			TaskID:    taskID,
			AgentID:   agentID,
			AgentRole: agentRole,
		}

		res, err := registry.Execute(ctx, name, call)
		if err != nil {
			return "Error: " + err.Error(), nil
		}
		if !res.Success {
			errMsg := "Error: tool execution failed"
			if res.Message != "" {
				// Tool/registry-level messages (e.g. Registry.Execute's authorization
				// rejection) already carry their own "Error: " prefix — don't double it.
				if strings.HasPrefix(res.Message, "Error: ") {
					errMsg = res.Message
				} else {
					errMsg = "Error: " + res.Message
				}
			}
			if len(res.Diagnostics) > 0 {
				var diagMsgs []string
				for _, d := range res.Diagnostics {
					diagMsgs = append(diagMsgs, fmt.Sprintf("- [%s] %s:%d: %s", d.Severity, d.File, d.Line, d.Message))
				}
				errMsg = errMsg + "\nDiagnostics:\n" + strings.Join(diagMsgs, "\n")
			}
			return errMsg, nil
		}
		return res.Output, nil
	}
}
