package orchestrator

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type statusUpdaterAdapter struct {
	update func(ctx context.Context, taskID string, status string) (*models.Task, error)
}

func (a statusUpdaterAdapter) UpdateTaskStatus(ctx context.Context, taskID string, status string) (*models.Task, error) {
	return a.update(ctx, taskID, status)
}

type llmRunnerAdapter struct {
	run func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (map[string]any, error)
}

func (a llmRunnerAdapter) RunLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepID string, instruction string) (steps.StepResult, error) {
	return a.run(ctx, task, agent, jobID, stepID, instruction)
}

type sandboxRunnerAdapter struct {
	run func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error)
}

func (a sandboxRunnerAdapter) RunCommand(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (steps.StepResult, error) {
	return a.run(ctx, task, agent, stepID, command)
}

type testerRunnerAdapter struct {
	run func(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error)
}

func (a testerRunnerAdapter) RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, stepName string, changedFiles []string, worktreeSuffix string) (steps.StepResult, error) {
	return a.run(ctx, task, agent, jobID, stepName, changedFiles, worktreeSuffix)
}

type artifactSaverAdapter struct {
	save func(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error
}

func (a artifactSaverAdapter) SaveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	return a.save(ctx, jobID, taskID, step, artType, payload)
}

type loggerAdapter struct {
	log func(ctx context.Context, taskID string, jobID *string, level string, message string)
}

func (a loggerAdapter) Log(ctx context.Context, taskID string, jobID *string, level string, message string) {
	a.log(ctx, taskID, jobID, level, message)
}

type promptAssemblerAdapter struct {
	assemble func(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error)
}

func (a promptAssemblerAdapter) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error) {
	return a.assemble(ctx, task, agent, history)
}

type traceRecorderAdapter struct {
	write func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed map[string]any)
}

func (a traceRecorderAdapter) WriteLLMCallTrace(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, messages []llm.Message, resp *llm.Response, parsed steps.StepResult) {
	a.write(ctx, task, agent, stepID, messages, resp, map[string]any(parsed))
}
