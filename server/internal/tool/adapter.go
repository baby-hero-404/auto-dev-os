package tool

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/skills"
)

// SkillExecutorAdapter adapts the new Registry to the legacy SkillExecutor.Execute signature.
type SkillExecutorAdapter struct {
	Registry *Registry
}

// NewSkillExecutorAdapter creates a new SkillExecutorAdapter wrapping a Registry.
func NewSkillExecutorAdapter(r *Registry) *SkillExecutorAdapter {
	return &SkillExecutorAdapter{Registry: r}
}

// Execute converts the legacy SkillCall to a tool.Call, runs the tool, and converts the tool.Result back.
func (a *SkillExecutorAdapter) Execute(ctx context.Context, call skills.SkillCall) skills.SkillResult {
	if a.Registry == nil {
		return skills.SkillResult{
			Name:    call.Name,
			Success: false,
			Error:   "tool registry is not configured",
		}
	}

	tc := Call{
		Input:     call.Input,
		Workspace: call.Workspace,
		TaskID:    call.TaskID,
		AgentID:   call.AgentID,
	}

	res, err := a.Registry.Execute(ctx, call.Name, tc)
	if err != nil {
		return skills.SkillResult{
			Name:    call.Name,
			Success: false,
			Error:   err.Error(),
		}
	}

	var errStr string
	if !res.Success {
		errStr = res.Message
		if errStr == "" && len(res.Diagnostics) > 0 {
			errStr = res.Diagnostics[0].Message
		}
		if errStr == "" {
			errStr = "tool execution failed"
		}
	}

	return skills.SkillResult{
		Name:    call.Name,
		Success: res.Success,
		Output:  res.Output,
		Error:   errStr,
	}
}
