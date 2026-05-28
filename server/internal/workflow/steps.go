package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// ──────────────────────────────────────────────────────────────────
// Step runners — concrete StepFunc implementations for each DAG node.
// Each returns map[string]any conforming to the step's OutputSchema.
// ──────────────────────────────────────────────────────────────────

// NewAnalyzeStep returns the step runner for task analysis.
// In Phase 3b this delegates to the heuristic classifier; future
// phases will invoke a real LLM planner agent.
func NewAnalyzeStep() StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		return map[string]any{
			"step":   "analyze",
			"status": "completed",
			"note":   "analysis delegated to task service heuristic",
		}, nil
	}
}

// NewPlanStep decomposes a task into sub-tasks. Currently a passthrough.
func NewPlanStep() StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		return map[string]any{
			"step":   "plan",
			"status": "completed",
			"note":   "task decomposition placeholder — single sub-task passthrough",
		}, nil
	}
}

// NewCodeStep runs agent commands inside the sandbox container.
func NewCodeStep(runtime sandbox.Runtime) StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		taskID, _ := sc.Initial["task_id"].(string)
		result, err := runtime.Run(ctx, sandbox.CommandRequest{
			TaskID:      taskID,
			Command:     []string{"auto-code-os-stub", "code", taskID},
			NetworkMode: sandbox.NetworkModeNone,
			Timeout:     5 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("step code sandbox: %w", err)
		}
		return map[string]any{
			"step":      "code",
			"status":    "completed",
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
		}, nil
	}
}

// NewMergeStep merges parallel code outputs. Currently a passthrough
// for single-branch sequential execution.
func NewMergeStep() StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		return map[string]any{
			"step":   "merge",
			"status": "completed",
			"note":   "single-branch merge passthrough — no conflicts",
		}, nil
	}
}

// NewReviewStep runs the reviewer agent in the sandbox.
func NewReviewStep(runtime sandbox.Runtime) StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		taskID, _ := sc.Initial["task_id"].(string)
		result, err := runtime.Run(ctx, sandbox.CommandRequest{
			TaskID:      taskID,
			Command:     []string{"auto-code-os-stub", "review", taskID},
			NetworkMode: sandbox.NetworkModeNone,
			Timeout:     3 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("step review sandbox: %w", err)
		}
		return map[string]any{
			"step":      "review",
			"status":    "completed",
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
		}, nil
	}
}

// NewFixStep runs the fix agent to address review feedback.
func NewFixStep(runtime sandbox.Runtime) StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		taskID, _ := sc.Initial["task_id"].(string)
		result, err := runtime.Run(ctx, sandbox.CommandRequest{
			TaskID:      taskID,
			Command:     []string{"auto-code-os-stub", "fix", taskID},
			NetworkMode: sandbox.NetworkModeNone,
			Timeout:     3 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("step fix sandbox: %w", err)
		}
		return map[string]any{
			"step":      "fix",
			"status":    "completed",
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
		}, nil
	}
}

// NewTestStep runs the QA test suite in the sandbox.
func NewTestStep(runtime sandbox.Runtime) StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		taskID, _ := sc.Initial["task_id"].(string)
		result, err := runtime.Run(ctx, sandbox.CommandRequest{
			TaskID:      taskID,
			Command:     []string{"auto-code-os-stub", "test", taskID},
			NetworkMode: sandbox.NetworkModeNone,
			Timeout:     5 * time.Minute,
		})
		if err != nil {
			return nil, fmt.Errorf("step test sandbox: %w", err)
		}
		passed := result.ExitCode == 0
		status := "completed"
		if !passed {
			status = "test_failed"
		}
		out := map[string]any{
			"step":      "test",
			"status":    status,
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
			"passed":    passed,
		}
		if !passed {
			return out, fmt.Errorf("test suite failed with exit code %d", result.ExitCode)
		}
		return out, nil
	}
}

// NewPRStep creates a pull request. Currently a placeholder.
func NewPRStep() StepFunc {
	return func(ctx context.Context, sc StepContext) (map[string]any, error) {
		return map[string]any{
			"step":   "pr",
			"status": "completed",
			"note":   "PR creation placeholder — will integrate with gitops.GitProvider",
		}, nil
	}
}

// ──────────────────────────────────────────────────────────────────
// DefaultWorkflowDefinition creates the 8-step default DAG definition
// with optional schema validation and complexity-based step skipping.
// ──────────────────────────────────────────────────────────────────

// DefaultWorkflowDefinition builds the canonical workflow for a task.
func DefaultWorkflowDefinition(runtime sandbox.Runtime, complexity string) Definition {
	statusSchema := Schema{Fields: map[string]FieldSchema{
		"step":   {Type: FieldString, Required: true},
		"status": {Type: FieldString, Required: true},
	}}

	steps := []StepDefinition{
		{ID: "analyze", Name: "Analyze", OutputSchema: statusSchema, Run: NewAnalyzeStep()},
		{ID: "plan", Name: "Plan", DependsOn: []string{"analyze"}, OutputSchema: statusSchema, Run: NewPlanStep()},
		{ID: "code", Name: "Code", DependsOn: []string{"plan"}, OutputSchema: statusSchema, Run: NewCodeStep(runtime)},
		{ID: "merge", Name: "Merge", DependsOn: []string{"code"}, OutputSchema: statusSchema, Run: NewMergeStep()},
		{ID: "review", Name: "Review", DependsOn: []string{"merge"}, OutputSchema: statusSchema, Run: NewReviewStep(runtime)},
		{ID: "fix", Name: "Fix", DependsOn: []string{"review"}, OutputSchema: statusSchema, Run: NewFixStep(runtime)},
		{ID: "test", Name: "Test", DependsOn: []string{"fix"}, OutputSchema: statusSchema, Run: NewTestStep(runtime)},
		{ID: "pr", Name: "PR", DependsOn: []string{"test"}, OutputSchema: statusSchema, Run: NewPRStep()},
	}

	// Easy tasks skip the review/fix loop.
	skip := DefaultStepComplexityFilter(complexity)
	if len(skip) > 0 {
		steps = filterSteps(steps, skip)
	}

	return Definition{Name: "auto-code-os-workflow", Steps: steps}
}

// DefaultStepComplexityFilter returns which steps to skip for easy tasks.
func DefaultStepComplexityFilter(complexity string) map[string]bool {
	skip := map[string]bool{}
	if complexity == models.TaskComplexityEasy {
		skip["review"] = true
		skip["fix"] = true
	}
	return skip
}

// filterSteps removes skipped steps and patches dependencies to bypass them.
func filterSteps(steps []StepDefinition, skip map[string]bool) []StepDefinition {
	// Build a lookup of what each skipped step depends on.
	depOf := map[string][]string{}
	for _, s := range steps {
		depOf[s.ID] = s.DependsOn
	}

	var result []StepDefinition
	for _, s := range steps {
		if skip[s.ID] {
			continue
		}
		// Patch any dependency on a skipped step: replace with the
		// skipped step's own dependencies (transitive bypass).
		var patched []string
		for _, dep := range s.DependsOn {
			if skip[dep] {
				patched = append(patched, depOf[dep]...)
			} else {
				patched = append(patched, dep)
			}
		}
		s.DependsOn = patched
		result = append(result, s)
	}
	return result
}
