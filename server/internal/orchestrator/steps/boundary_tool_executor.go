package steps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// editCapabilityTools are the tool names that mutate files on disk. Every call to one of these
// must pass the same execution-boundary policy check the diff-based flow already enforces via
// patch.EvaluatePolicy, before the edit is allowed to run (Issue 1+2: enabling native tool calls
// for coding steps must not regress the security boundary enforcement described in the report).
var editCapabilityTools = map[string]bool{
	"search_replace": true,
	"create_file":    true,
}

// NewBoundaryCheckedToolExecutor wraps NewRegistryToolExecutor so that edit-capability tool
// calls are evaluated against the task's execution boundaries first:
//   - SeverityCritical aborts the whole agentic loop and pauses the task for human review,
//     exactly like a critical PolicyViolationError does in the diff-based path today.
//   - SeverityError is fed back to the LLM as a tool error so it can pick a different file.
//   - SeverityWarning/SeverityInfo are allowed and recorded as ExpandedBoundaries, mirroring
//     patch.Runner.ApplyPatch's auto-expansion behavior.
func NewBoundaryCheckedToolExecutor(registry *tool.Registry, workspace string, task *models.Task, agentID, agentRole string, tasks TaskRepository) llmrunner.ToolExecutor {
	base := NewRegistryToolExecutor(registry, workspace, task.ID, agentID, agentRole)

	return func(ctx context.Context, name, argumentsJSON string) (string, error) {
		if editCapabilityTools[name] {
			var args map[string]any
			_ = json.Unmarshal([]byte(argumentsJSON), &args)
			path, _ := args["path"].(string)
			if path != "" {
				var analysis models.TaskAnalysis
				if len(task.Analysis) > 0 {
					_ = json.Unmarshal(task.Analysis, &analysis)
				}

				oldFile := path
				if name == "create_file" {
					oldFile = "/dev/null"
				}

				decision := patch.EvaluatePolicy(path, oldFile, &analysis)
				switch decision.Severity {
				case patch.SeverityCritical:
					return "", fmt.Errorf("%w: security boundary violation on %q: %s", workflow.ErrPaused, path, decision.Reason)
				case patch.SeverityError:
					return fmt.Sprintf("Error: execution boundary violation on %q: %s. Choose a file within your assigned module, or explain in your summary why this file must change.", path, decision.Reason), nil
				case patch.SeverityWarning, patch.SeverityInfo:
					if tasks != nil {
						_ = updateTaskAnalysis(ctx, task.ID, tasks, task, func(a *models.TaskAnalysis) bool {
							a.ExpandedBoundaries = append(a.ExpandedBoundaries, models.ExpandedBoundary{
								File:       path,
								Reason:     decision.Reason,
								Capability: decision.Capability,
								Risk:       decision.Risk,
							})
							return true
						})
					}
				}
			}
		}
		return base(ctx, name, argumentsJSON)
	}
}
