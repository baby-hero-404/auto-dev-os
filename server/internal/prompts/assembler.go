package prompts

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type PromptAssembler struct {
	rules       *repository.RuleRepo
	skills      SkillLister
	baseTools   []llm.ToolDefinition
	promptPaths paths.PromptPaths
	fs          paths.FileSystem
	dataRoot    string
	ctxEngine   provider.ContextEngine
}

type SkillLister interface {
	List(context.Context) ([]models.Skill, error)
}

func NewPromptAssembler(baseTools []llm.ToolDefinition, ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{
		baseTools:   baseTools,
		promptPaths: paths.NewOSPromptPaths("."),
		fs:          paths.NewOSFileSystem(),
		ctxEngine:   ctxEngine,
	}
}

func NewPromptAssemblerWithRules(rules *repository.RuleRepo, baseTools []llm.ToolDefinition, promptPaths paths.PromptPaths, fs paths.FileSystem, ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{
		rules:       rules,
		baseTools:   baseTools,
		promptPaths: promptPaths,
		fs:          fs,
		ctxEngine:   ctxEngine,
	}
}

func (a *PromptAssembler) WithSkillLister(skills SkillLister) *PromptAssembler {
	a.skills = skills
	return a
}

func (a *PromptAssembler) WithDataRoot(dataRoot string) *PromptAssembler {
	a.dataRoot = dataRoot
	return a
}

func (a *PromptAssembler) Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error) {
	return a.AssembleForAgent(ctx, task, nil, nil)
}

type contextKey string

const MemoriesCtxKey contextKey = "retrieved_memories"
const StepIDCtxKey contextKey = "prompt_step_id"
const StepInputsCtxKey contextKey = "step_inputs"

func stepIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(StepIDCtxKey).(string); ok {
		return v
	}
	return ""
}

func StepInputsFromCtx(ctx context.Context) map[string]map[string]any {
	if v, ok := ctx.Value(StepInputsCtxKey).(map[string]map[string]any); ok {
		return v
	}
	return nil
}

const BudgetLogCtxKey contextKey = "budget_log_entries"

type BudgetTrace struct {
	Logs []string
}

func WithBudgetTrace(ctx context.Context) (context.Context, *BudgetTrace) {
	trace := &BudgetTrace{}
	return context.WithValue(ctx, BudgetLogCtxKey, trace), trace
}

func BudgetTraceFromCtx(ctx context.Context) *BudgetTrace {
	if t, ok := ctx.Value(BudgetLogCtxKey).(*BudgetTrace); ok {
		return t
	}
	return nil
}

// shouldInjectFullSpec returns true for steps that need the full OpenSpec
// (analyze, plan, review). Coding and fix steps already get the relevant
// subtask text injected by the step runner itself.
func shouldInjectFullSpec(stepID string) bool {
	return stepID == "" ||
		stepID == workflow.StepAnalyze ||
		stepID == workflow.StepPlan ||
		stepID == workflow.StepContextLoad
}

// isCodingStep returns true for steps that produce code patches.
func isCodingStep(stepID string) bool {
	return strings.HasPrefix(stepID, workflow.StepCodeBackend) ||
		strings.HasPrefix(stepID, workflow.StepCodeFrontend) ||
		stepID == workflow.StepFix
}

func (a *PromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error) {
	sections, err := a.collect(ctx, task, agent)
	if err != nil {
		return nil, nil, err
	}

	// Hard budget enforcement (Target: 8192 tokens)
	sections = a.optimizeBudget(ctx, sections, 8192)

	// Sort sections logically based on RenderOrder
	sorted := a.sort(sections)

	// Render compiled system and user prompt strings
	system, user := a.render(sorted)

	// Dynamic Metadata Injection (appendSystemPrompt)
	metadata := map[string]any{
		"project_id": task.ProjectID,
		"task_id":    task.ID,
	}
	if agent != nil {
		metadata["assigned_role"] = agent.Role
	}
	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	if len(analysis.TaskRules) > 0 {
		metadata["task_rules"] = analysis.TaskRules
	}
	system = appendSystemPrompt(system, metadata)

	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	messages = append(messages, TruncateHistory(history, 12000)...)

	// Fetch dynamic tools dynamically resolved from JIT skills
	tools, err := a.toolDefinitionsForAgent(ctx, task, agent)
	if err != nil {
		return nil, nil, err
	}

	return messages, tools, nil
}
