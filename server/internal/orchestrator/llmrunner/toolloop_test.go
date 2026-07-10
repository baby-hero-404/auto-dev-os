package llmrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

func TestRunToolLoop_ExecutesToolCallsThenAccepts(t *testing.T) {
	calls := 0
	var executedTool string

	cfg := ToolLoopConfig{
		Messages: []llm.Message{{Role: "user", Content: "do the thing"}},
		Tools:    []llm.ToolDefinition{{Name: "read_file"}},
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			if calls == 1 {
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "read_file", Arguments: `{"path":"a.go"}`}},
				}, nil
			}
			return &llm.Response{Content: `{"summary":"done"}`}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			executedTool = name
			return "file contents", nil
		},
	}

	parsed, messages, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 LLM calls (one tool call round, one final), got %d", calls)
	}
	if executedTool != "read_file" {
		t.Errorf("expected read_file to be executed, got %q", executedTool)
	}
	if parsed["summary"] != "done" {
		t.Errorf("expected parsed summary 'done', got %v", parsed)
	}
	// tool result message must be present with role "tool"
	foundToolMsg := false
	for _, m := range messages {
		if m.Role == "tool" && m.Content == "file contents" {
			foundToolMsg = true
		}
	}
	if !foundToolMsg {
		t.Errorf("expected a tool result message in final messages, got %+v", messages)
	}
}

func TestRunToolLoop_ValidateRejectsThenAccepts(t *testing.T) {
	calls := 0
	cfg := ToolLoopConfig{
		Messages: []llm.Message{{Role: "user", Content: "do it"}},
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			if calls == 1 {
				return &llm.Response{Content: `{"foo":"bar"}`}, nil
			}
			return &llm.Response{Content: `{"summary":"done"}`}, nil
		},
		Validate: func(parsed map[string]any) error {
			if _, ok := parsed["summary"]; !ok {
				return errors.New("missing summary field")
			}
			return nil
		},
	}

	parsed, _, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected validation failure to trigger a second call, got %d calls", calls)
	}
	if parsed["summary"] != "done" {
		t.Errorf("expected final parsed summary 'done', got %v", parsed)
	}
}

func TestRunToolLoop_ExhaustsIterationBudget(t *testing.T) {
	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		MaxIterations: 2,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			return &llm.Response{Content: `not json`}, nil
		},
	}

	_, _, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected an error when the iteration budget is exhausted")
	}
}

func TestRunToolLoop_ToolExecutorErrorAbortsImmediately(t *testing.T) {
	calls := 0
	cfg := ToolLoopConfig{
		Messages: []llm.Message{{Role: "user", Content: "do it"}},
		Tools:    []llm.ToolDefinition{{Name: "search_replace"}},
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			return &llm.Response{
				ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "search_replace", Arguments: `{"path":"../../etc/passwd"}`}},
			}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return "", errors.New("security boundary violation: paused for human review")
		},
	}

	_, _, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected tool executor error to abort the loop")
	}
	if calls != 1 {
		t.Errorf("expected the loop to stop after the first tool executor error, got %d chat calls", calls)
	}
}

func TestRunToolLoop_ChatErrorPropagates(t *testing.T) {
	cfg := ToolLoopConfig{
		Messages: []llm.Message{{Role: "user", Content: "do it"}},
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			return nil, errors.New("provider down")
		},
	}

	_, _, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected chat error to propagate")
	}
}
