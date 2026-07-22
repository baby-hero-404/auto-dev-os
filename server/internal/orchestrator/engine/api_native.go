package engine

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// Delegate runs the existing API-native LLM tool-loop for one coding step.
// It is supplied by the orchestrator so this package never depends on the
// LLM runner directly.
type Delegate func(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error)

// apiNativeEngine is a thin passthrough adapter: it changes nothing about
// existing behavior, it just lets callers dispatch through the same
// ExecutionEngine interface regardless of which engine is resolved.
type apiNativeEngine struct {
	delegate Delegate
}

// NewAPINativeEngine wraps an existing coding-step implementation so it can
// be dispatched through the ExecutionEngine interface unchanged.
func NewAPINativeEngine(delegate Delegate) ExecutionEngine {
	return &apiNativeEngine{delegate: delegate}
}

func (e *apiNativeEngine) Name() string { return models.ExecutionEngineAPINative }

func (e *apiNativeEngine) Preflight(ctx context.Context, req CodeStepRequest) (string, error) {
	return "", nil
}

func (e *apiNativeEngine) RunCodeStep(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error) {
	if e.delegate == nil {
		return nil, fmt.Errorf("api_native engine: no delegate configured")
	}
	return e.delegate(ctx, req)
}
