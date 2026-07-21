package llmrunner

import (
	"context"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestToolLoop_NegativeMemory(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "do it"}}
	tools := []llm.ToolDefinition{{Name: "run_tests"}}

	// Fake chat that calls a tool once, then finishes
	callCount := 0
	chatFunc := func(ctx context.Context, msgs []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
		callCount++
		if callCount == 1 {
			return &llm.Response{
				Content: "",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "run_tests", Arguments: `{"command":"go test"}`},
				},
			}, nil
		}
		return &llm.Response{
			Content: `{"summary": "done"}`,
		}, nil
	}

	execToolFunc := func(ctx context.Context, name string, argumentsJSON string) (string, error) {
		return "Error: execution failed terribly\nsome other details", nil
	}

	cfg := ToolLoopConfig{
		Messages:      messages,
		Tools:         tools,
		MaxIterations: 3,
		Chat:          chatFunc,
		ExecuteTool:   execToolFunc,
		Validate: func(parsed map[string]any) error {
			return nil
		},
	}

	_, _, partial, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(partial.FailedCalls) != 1 {
		t.Fatalf("expected 1 failed call, got %d", len(partial.FailedCalls))
	}

	expectedPrefix := "run_tests({\"command\":\"go test\"}) - Error: execution failed terribly"
	if !strings.Contains(partial.FailedCalls[0], expectedPrefix) {
		t.Errorf("expected failed call to contain %q, got: %q", expectedPrefix, partial.FailedCalls[0])
	}
}

func TestShadowStateMachine_HardGating(t *testing.T) {
	budget := models.PhaseBudgets{Discovery: 1, Implementation: 1, Validation: 1}

	// Create StateMachine directly in Failed state
	sm := NewStateMachineFrom(StateFailed, budget)

	err := sm.CheckTool("create_file")
	if err == nil {
		t.Error("expected CheckTool to fail in Failed state")
	}
}
