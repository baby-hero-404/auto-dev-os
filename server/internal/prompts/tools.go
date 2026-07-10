package prompts

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (a *PromptAssembler) toolDefinitionsForAgent(ctx context.Context, task models.Task, agent *models.Agent, dynamicTools []llm.ToolDefinition) ([]llm.ToolDefinition, error) {
	if len(dynamicTools) > 0 {
		return dynamicTools, nil
	}
	if a != nil {
		return a.baseTools, nil
	}
	return []llm.ToolDefinition{}, nil
}
