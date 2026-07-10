package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/llmrunner"
	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) runLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	o.initCheckpoints()
	var assemble llmrunner.PromptAssembler
	if o.prompts != nil {
		var budgetTrace *prompts.BudgetTrace
		ctx, budgetTrace = prompts.WithBudgetTrace(ctx)
		_ = budgetTrace
		assemble = func(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, error) {
			var tools []llm.ToolDefinition
			if o.capManager != nil && agent != nil {
				tools = o.capManager.ToolsForRole(agent.Role)
			}
			messages, _, err := o.prompts.AssembleForAgent(ctx, task, agent, history, tools)
			return messages, err
		}
	}
	runner := llmrunner.Runner{
		WorkspaceRoot:           o.workspaceRoot,
		Provider:                o.llm,
		AssemblePrompt:          assemble,
		Projects:                o.projects,
		ReadAffectedFileContent: o.readAffectedFileContent,
		SaveArtifact:            o.checkpoints.SaveArtifact,
		WriteTrace:              o.writeLLMCallTrace,
		Log:                     o.log,
	}
	return runner.Run(ctx, task, agent, jobID, stepID, instruction)
}
