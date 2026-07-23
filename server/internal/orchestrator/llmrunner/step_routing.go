package llmrunner

import (
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// stepBaseLevel is the default step-routing matrix (REQ-002): steps that are
// cheap to run at a lower model level than the project's default. Steps not
// listed here (code_*, fix, cli_implement, ...) resolve to the project's
// DefaultModelLevel unchanged, since those are the steps that actually write
// code and benefit least from a cheaper model.
var stepBaseLevel = map[string]string{
	workflow.StepContextLoad: llm.LevelFast,
	workflow.StepAnalyze:     llm.LevelFast,
	workflow.StepCLIAnalyze:  llm.LevelFast,
	workflow.StepPlan:        llm.LevelBalanced,
	workflow.StepReview:      llm.LevelBalanced,
	workflow.StepCrossReview: llm.LevelBalanced,
	workflow.StepCLISpec:     llm.LevelBalanced,
}

var levelOrder = map[string]int{llm.LevelFast: 0, llm.LevelBalanced: 1, llm.LevelPowerful: 2}

// downgradeLevel returns the next tier down from level (powerful->balanced,
// balanced->fast); fast (or an unrecognized level) is returned unchanged.
func downgradeLevel(level string) string {
	switch level {
	case llm.LevelPowerful:
		return llm.LevelBalanced
	case llm.LevelBalanced:
		return llm.LevelFast
	default:
		return level
	}
}

// ResolveStepModelLevel implements the smart-router matrix (REQ-002/003/004/M01).
//
//   - smartRouting == false, or an empty projectLevel: returns projectLevel
//     unchanged — every step behaves exactly as it did before this feature (REQ-M01).
//   - Steps present in stepBaseLevel resolve to their matrix level, but never
//     exceed projectLevel (a project pinned to "balanced" never runs a step at
//     "powerful" just because the matrix says so). Steps absent from the
//     matrix (code_*, fix, ...) use projectLevel as-is.
//   - A task with Complexity == "easy" downgrades the resolved level by one
//     tier (REQ-003).
//   - isRetry (a failing step being retried, e.g. by the patch-retry loop)
//     restores the pre-downgrade level, so a cheap model doesn't loop forever
//     on a failure it can't fix (REQ-004).
func ResolveStepModelLevel(stepID, projectLevel, complexity string, isRetry bool, smartRouting bool) string {
	if !smartRouting || projectLevel == "" {
		return projectLevel
	}

	resolved := projectLevel
	if base, matrixed := stepBaseLevel[stepID]; matrixed {
		resolved = base
		if levelOrder[resolved] > levelOrder[projectLevel] {
			resolved = projectLevel
		}
	}

	if strings.EqualFold(complexity, models.TaskComplexityEasy) {
		if isRetry {
			return resolved
		}
		return downgradeLevel(resolved)
	}

	return resolved
}
