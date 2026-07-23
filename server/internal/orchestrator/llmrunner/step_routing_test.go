package llmrunner

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestResolveStepModelLevel_MatrixStepDowngradesBelowProjectLevel(t *testing.T) {
	got := ResolveStepModelLevel(workflow.StepAnalyze, llm.LevelPowerful, models.TaskComplexityMedium, false, true)
	if got != llm.LevelFast {
		t.Errorf("expected analyze step to resolve to fast, got %s", got)
	}
}

func TestResolveStepModelLevel_CodeStepUsesProjectLevelUnchanged(t *testing.T) {
	got := ResolveStepModelLevel(workflow.StepCodeBackend, llm.LevelPowerful, models.TaskComplexityMedium, false, true)
	if got != llm.LevelPowerful {
		t.Errorf("expected code_backend to resolve to project level (powerful), got %s", got)
	}
}

func TestResolveStepModelLevel_MatrixNeverExceedsProjectLevel(t *testing.T) {
	// review's matrix level is "balanced", but a project pinned to "fast" must
	// never see anything higher than fast for any step.
	got := ResolveStepModelLevel(workflow.StepReview, llm.LevelFast, models.TaskComplexityMedium, false, true)
	if got != llm.LevelFast {
		t.Errorf("expected review step to be clamped to project level (fast), got %s", got)
	}
}

func TestResolveStepModelLevel_EasyComplexityDowngradesOneTier(t *testing.T) {
	// code_backend resolves to projectLevel (powerful); easy complexity should
	// downgrade it one tier to balanced.
	got := ResolveStepModelLevel(workflow.StepCodeBackend, llm.LevelPowerful, models.TaskComplexityEasy, false, true)
	if got != llm.LevelBalanced {
		t.Errorf("expected easy-complexity downgrade to balanced, got %s", got)
	}
}

func TestResolveStepModelLevel_RetryRestoresPreDowngradeLevel(t *testing.T) {
	// Same setup as above, but this is a retry attempt: the escape hatch
	// should restore the pre-downgrade level instead of the cheaper one.
	got := ResolveStepModelLevel(workflow.StepCodeBackend, llm.LevelPowerful, models.TaskComplexityEasy, true, true)
	if got != llm.LevelPowerful {
		t.Errorf("expected retry to restore pre-downgrade level (powerful), got %s", got)
	}
}

func TestResolveStepModelLevel_SmartRoutingOffIsNoOp(t *testing.T) {
	got := ResolveStepModelLevel(workflow.StepAnalyze, llm.LevelPowerful, models.TaskComplexityEasy, false, false)
	if got != llm.LevelPowerful {
		t.Errorf("expected smart_routing=false to return projectLevel unchanged, got %s", got)
	}
}

func TestResolveStepModelLevel_EmptyProjectLevelIsNoOp(t *testing.T) {
	got := ResolveStepModelLevel(workflow.StepAnalyze, "", models.TaskComplexityEasy, false, true)
	if got != "" {
		t.Errorf("expected empty projectLevel to pass through unchanged, got %s", got)
	}
}
