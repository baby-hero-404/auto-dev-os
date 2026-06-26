package steps

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// StepResult is the canonical return type from step execution.
type StepResult = map[string]any

// StepRuntime carries job-scoped context that every step needs.
// This is injected at construction time (steps are job-scoped),
// so Execute does not need task/agent/jobID parameters.
type StepRuntime struct {
	Task  *models.Task
	Agent *models.Agent
	JobID string
}

// Step is the contract every workflow step must implement.
// Steps are constructed per-job with their specific dependencies
// and StepRuntime, so Execute only needs ctx and workflow context.
type Step interface {
	// ID returns the workflow step identifier (e.g. "context_load").
	ID() string

	// Execute runs the step logic and returns a result.
	Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error)

	// StatusOnResume returns the task status to restore when this step
	// is skipped during checkpoint recovery. It receives the cached
	// checkpoint output so steps like review can choose between
	// "testing" and "fixing" based on findings. Empty string means
	// no status transition needed.
	StatusOnResume(output StepResult) string
}
