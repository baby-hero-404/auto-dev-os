package prompts

import (
	"context"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type PromptAssembler struct {
	rules         *repository.RuleRepo
	skills        SkillLister
	learnedSkills LearnedSkillsLister
	promptPaths   paths.PromptPaths
	fs            paths.FileSystem
	dataRoot      string
	ctxEngine     provider.ContextEngine
}

type SkillLister interface {
	List(context.Context) ([]models.Skill, error)
}

func NewPromptAssembler(ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{
		promptPaths: paths.NewOSPromptPaths("."),
		fs:          paths.NewOSFileSystem(),
		ctxEngine:   ctxEngine,
	}
}

func NewPromptAssemblerWithRules(rules *repository.RuleRepo, promptPaths paths.PromptPaths, fs paths.FileSystem, ctxEngine provider.ContextEngine) *PromptAssembler {
	return &PromptAssembler{
		rules:       rules,
		promptPaths: promptPaths,
		fs:          fs,
		ctxEngine:   ctxEngine,
	}
}

func (a *PromptAssembler) WithSkillLister(skills SkillLister) *PromptAssembler {
	a.skills = skills
	return a
}

func (a *PromptAssembler) WithLearnedSkillsLister(ls LearnedSkillsLister) *PromptAssembler {
	a.learnedSkills = ls
	return a
}

func (a *PromptAssembler) WithDataRoot(dataRoot string) *PromptAssembler {
	a.dataRoot = dataRoot
	return a
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
