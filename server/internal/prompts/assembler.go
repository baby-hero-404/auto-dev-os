package prompts

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type PromptAssembler struct {
	rules            *repository.RuleRepo
	skills           SkillLister
	baseTools        []llm.ToolDefinition
	promptPaths      paths.PromptPaths
	fs               paths.FileSystem
	dataRoot         string
	ctxEngine        provider.ContextEngine
	metadataProvider llm.MetadataProvider
}

// defaultPromptBudget is used when no model metadata is available to derive a budget from.
const defaultPromptBudget = 8192

// promptBudgetReserveRatio is the fraction of the model's context window reserved for the prompt
// (the remainder is left for the model's output).
const promptBudgetReserveRatio = 0.7

// SetModelMetadataProvider wires the LLM provider/gateway used to derive the token budget from
// the target model's max context window instead of a hardcoded constant.
func (a *PromptAssembler) SetModelMetadataProvider(mp llm.MetadataProvider) {
	a.metadataProvider = mp
}

func (a *PromptAssembler) resolvePromptBudget() int {
	if a.metadataProvider == nil {
		return defaultPromptBudget
	}
	meta := a.metadataProvider.Metadata()
	if meta.MaxContextTokens <= 0 {
		return defaultPromptBudget
	}
	budget := int(float64(meta.MaxContextTokens) * promptBudgetReserveRatio)
	if budget <= 0 {
		return defaultPromptBudget
	}
	return budget
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

func (a *PromptAssembler) SetBaseTools(tools []llm.ToolDefinition) {
	a.baseTools = tools
}

func (a *PromptAssembler) Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error) {
	return a.AssembleForAgent(ctx, task, nil, nil, nil)
}

type contextKey string

const MemoriesCtxKey contextKey = "retrieved_memories"
const StepIDCtxKey contextKey = "prompt_step_id"
const StepInputsCtxKey contextKey = "step_inputs"
const IsRetryCtxKey contextKey = "is_retry"
const UseSearchReplaceCtxKey contextKey = "use_search_replace"

func IsRetry(ctx context.Context) bool {
	return ctx.Value(IsRetryCtxKey) == true
}

func UseSearchReplace(ctx context.Context) bool {
	return ctx.Value(UseSearchReplaceCtxKey) == true
}

func WithRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, IsRetryCtxKey, true)
}

func WithSearchReplace(ctx context.Context) context.Context {
	return context.WithValue(ctx, UseSearchReplaceCtxKey, true)
}

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
	Logs       []string
	ToolTokens int
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

func (a *PromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message, dynamicTools []llm.ToolDefinition) ([]llm.Message, []llm.ToolDefinition, error) {
	// Fetch dynamic tools dynamically resolved from JIT skills
	tools, err := a.toolDefinitionsForAgent(ctx, task, agent, dynamicTools)
	if err != nil {
		return nil, nil, err
	}

	promptBudget := a.resolvePromptBudget()

	sections, err := a.collect(ctx, task, agent, tools, promptBudget)
	if err != nil {
		return nil, nil, err
	}

	// Hard budget enforcement, derived from the target model's max context window (Issue 6)
	sections = a.optimizeBudget(ctx, sections, promptBudget)

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
		if err := json.Unmarshal(task.Analysis, &analysis); err != nil {
			slog.Warn("AssemblePrompt: failed to unmarshal task.Analysis, task_rules metadata will be omitted", "task_id", task.ID, "error", err)
		}
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

	return messages, tools, nil
}

func (a *PromptAssembler) ListAllSkills(ctx context.Context, task models.Task) ([]llm.ToolDefinition, error) {
	skills, err := a.loadAllSkills(ctx, task)
	if err != nil {
		return nil, err
	}
	var defs []llm.ToolDefinition
	for _, s := range skills {
		defs = append(defs, llm.ToolDefinition{
			Name:        s.Name,
			Description: s.Description,
		})
	}
	return defs, nil
}
