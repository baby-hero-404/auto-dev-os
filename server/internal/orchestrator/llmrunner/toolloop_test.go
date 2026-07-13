package llmrunner

import (
	"context"
	"errors"
	"strings"
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

	parsed, messages, _, err := RunToolLoop(context.Background(), cfg)
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

	parsed, _, _, err := RunToolLoop(context.Background(), cfg)
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

	_, _, _, err := RunToolLoop(context.Background(), cfg)
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

	_, _, _, err := RunToolLoop(context.Background(), cfg)
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

	_, _, _, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected chat error to propagate")
	}
}

func TestRunToolLoop_CircuitBreaker(t *testing.T) {
	calls := 0
	toolExecCalls := 0
	var lastToolResult string

	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "search_replace"}},
		MaxIterations: 5,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			if calls <= 3 {
				// LLM keeps calling search_replace on nonexistent.go
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "search_replace", Arguments: `{"path":"nonexistent.go"}`}},
				}, nil
			}
			return &llm.Response{Content: `{"summary":"done"}`}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			toolExecCalls++
			return "Error: File \"nonexistent.go\" does not exist.", nil
		},
	}

	parsed, messages, _, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The first 2 calls call ExecuteTool and fail.
	// The 3rd call hits circuit-breaker, so it is skipped (ExecuteTool is NOT called).
	if toolExecCalls != 2 {
		t.Errorf("expected ExecuteTool to be called exactly 2 times, got %d", toolExecCalls)
	}

	// The 4th chat call should return success, so total calls is 4
	if calls != 4 {
		t.Errorf("expected 4 chat calls, got %d", calls)
	}

	// Verify the corrective message is injected
	foundCorrective := false
	for _, m := range messages {
		if m.Role == "tool" && strings.Contains(m.Content, "multiple times without success") {
			foundCorrective = true
			lastToolResult = m.Content
			break
		}
	}
	if !foundCorrective {
		t.Errorf("expected corrective message to be injected in tool results")
	}
	if !strings.Contains(lastToolResult, "create_file") {
		t.Errorf("expected corrective message to contain 'create_file', got %q", lastToolResult)
	}
	if parsed["summary"] != "done" {
		t.Errorf("expected parsed summary 'done', got %v", parsed)
	}
}

// TestRunToolLoop_CircuitBreaker_NeverGivesUpTerminatesAtMaxIterations reproduces the
// scenario the "i--" loophole used to hide: a model that keeps re-issuing the exact same
// already-blocked call must still hit maxIterations, not loop indefinitely.
func TestRunToolLoop_CircuitBreaker_NeverGivesUpTerminatesAtMaxIterations(t *testing.T) {
	calls := 0
	toolExecCalls := 0

	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "search_replace"}},
		MaxIterations: 5,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			// The model NEVER gives up and NEVER produces a final answer — it keeps
			// calling search_replace on the same nonexistent path forever.
			return &llm.Response{
				ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "search_replace", Arguments: `{"path":"nonexistent.go"}`}},
			}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			toolExecCalls++
			return "Error: File \"nonexistent.go\" does not exist.", nil
		},
	}

	_, _, _, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected the loop to terminate with an error once maxIterations is reached")
	}
	if !strings.Contains(err.Error(), "exceeded max iterations") {
		t.Fatalf("expected an iteration-budget-exceeded error, got: %v", err)
	}
	// Exactly MaxIterations chat calls — the loop must NOT run past this just because
	// later rounds were all circuit-breaker-blocked (the old "i--" bug would have let
	// this run forever).
	if calls != 5 {
		t.Errorf("expected exactly 5 chat calls (MaxIterations), got %d", calls)
	}
	// Only the first 2 calls actually reach the tool executor; the rest are blocked.
	if toolExecCalls != 2 {
		t.Errorf("expected ExecuteTool to be called exactly 2 times before the breaker engages, got %d", toolExecCalls)
	}
}

// TestRunToolLoop_CircuitBreaker_ThrottlesPathlessTools verifies the breaker also
// applies to tools with no "path" argument (e.g. run_tests/run_build), keyed on their
// "command" argument instead.
func TestRunToolLoop_CircuitBreaker_ThrottlesPathlessTools(t *testing.T) {
	calls := 0
	toolExecCalls := 0

	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "run_tests"}},
		MaxIterations: 5,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			return &llm.Response{
				ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "run_tests", Arguments: `{"command":"go test ./broken/..."}`}},
			}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			toolExecCalls++
			return "Error: build failed, package does not compile.", nil
		},
	}

	_, _, _, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected the loop to terminate once maxIterations is reached")
	}
	if toolExecCalls != 2 {
		t.Errorf("expected the path-less tool to be throttled after 2 failures, got %d executions", toolExecCalls)
	}
	if calls != 5 {
		t.Errorf("expected exactly 5 chat calls, got %d", calls)
	}
}

// TestRunToolLoop_ExhaustionWithEditsAppliedReturnsPartialResult verifies that when the
// iteration budget runs out but at least one edit call succeeded, RunToolLoop returns a
// partial ToolLoopResult (nil error) instead of a hard failure (Issue 6).
func TestRunToolLoop_ExhaustionWithEditsAppliedReturnsPartialResult(t *testing.T) {
	calls := 0
	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "search_replace"}, {Name: "read_file"}},
		MaxIterations: 3,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			switch calls {
			case 1:
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "read_file", Arguments: `{"path":"a.go"}`}},
				}, nil
			default:
				// The model never produces a final answer — keeps editing forever.
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-n", Name: "search_replace", Arguments: `{"path":"a.go"}`}},
				}, nil
			}
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return "ok", nil
		},
	}

	parsed, _, result, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected nil error for a partial result, got: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil parsed JSON for a partial result, got: %v", parsed)
	}
	if result == nil || !result.Partial {
		t.Fatalf("expected a partial ToolLoopResult, got: %+v", result)
	}
	if len(result.EditsApplied) != 2 || result.EditsApplied[0] != "a.go" {
		t.Errorf("expected 2 edits applied to a.go, got: %v", result.EditsApplied)
	}
	if len(result.FilesRead) != 1 || result.FilesRead[0] != "a.go" {
		t.Errorf("expected a.go to be recorded as read, got: %v", result.FilesRead)
	}
}

// TestRunToolLoop_ExhaustionWithNoEditsReturnsHardError verifies the pre-existing behavior is
// preserved: exhaustion with zero successful edits is still a hard failure, not a partial result.
func TestRunToolLoop_ExhaustionWithNoEditsReturnsHardError(t *testing.T) {
	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		MaxIterations: 2,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			return &llm.Response{Content: `not json`}, nil
		},
	}

	_, _, result, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected a hard error when no edits were applied")
	}
	if result != nil && result.Partial {
		t.Errorf("did not expect a partial result when no edits were applied, got: %+v", result)
	}
}

// TestRunToolLoop_TruncatesLargeToolResult verifies a tool result exceeding maxToolResultChars
// (e.g. a large run_tests stdout+stderr blob) is truncated with a visible marker before being
// appended to message history, and the loop still completes successfully (Issue 7).
func TestRunToolLoop_TruncatesLargeToolResult(t *testing.T) {
	hugeOutput := strings.Repeat("x", maxToolResultChars+2000)

	calls := 0
	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "run_tests"}},
		MaxIterations: 3,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			if calls == 1 {
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "run_tests", Arguments: `{"command":"go test ./..."}`}},
				}, nil
			}
			return &llm.Response{Content: `{"summary":"done"}`}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return hugeOutput, nil
		},
	}

	parsed, messages, _, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected the loop to complete successfully despite a large tool result, got: %v", err)
	}
	if parsed["summary"] != "done" {
		t.Errorf("expected parsed summary 'done', got %v", parsed)
	}

	var toolMsg string
	for _, m := range messages {
		if m.Role == "tool" {
			toolMsg = m.Content
			break
		}
	}
	if toolMsg == "" {
		t.Fatal("expected a tool result message in the final messages")
	}
	if len(toolMsg) >= len(hugeOutput) {
		t.Errorf("expected the appended tool message to be shorter than the original %d-char output, got %d chars", len(hugeOutput), len(toolMsg))
	}
	if !strings.Contains(toolMsg, "truncated") {
		t.Errorf("expected a visible truncation marker in the appended tool message, got: %q", toolMsg[len(toolMsg)-60:])
	}
}

// TestRunToolLoop_ReadFileMemoization verifies a repeated read_file call on an already-read
// (path, line-range) within the same loop returns a short "already read" note instead of the
// full content again, without re-invoking the underlying tool (Issue 7).
func TestRunToolLoop_ReadFileMemoization(t *testing.T) {
	calls := 0
	readExecutions := 0

	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "read_file"}},
		MaxIterations: 4,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			if calls <= 2 {
				// Same path AND same line range both times.
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call", Name: "read_file", Arguments: `{"path":"a.go","start_line":1,"end_line":50}`}},
				}, nil
			}
			return &llm.Response{Content: `{"summary":"done"}`}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			readExecutions++
			return "the full file contents", nil
		},
	}

	_, messages, _, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readExecutions != 1 {
		t.Errorf("expected read_file to actually execute only once for a repeated (path, range), got %d executions", readExecutions)
	}

	var memoMsg string
	fullContentCount := 0
	for _, m := range messages {
		if m.Role != "tool" {
			continue
		}
		if m.Content == "the full file contents" {
			fullContentCount++
		}
		if strings.Contains(m.Content, "Already read at turn") {
			memoMsg = m.Content
		}
	}
	if fullContentCount != 1 {
		t.Errorf("expected the full content to appear exactly once in message history, got %d times", fullContentCount)
	}
	if memoMsg == "" {
		t.Error("expected an 'already read' note for the repeated call")
	}
}

// TestRunToolLoop_ReadFileMemoization_DifferentRangeNotMemoized verifies memoization is keyed
// on (path, line-range), not path alone — a different range on the same path must still execute.
func TestRunToolLoop_ReadFileMemoization_DifferentRangeNotMemoized(t *testing.T) {
	calls := 0
	readExecutions := 0

	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "do it"}},
		Tools:         []llm.ToolDefinition{{Name: "read_file"}},
		MaxIterations: 4,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			switch calls {
			case 1:
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "read_file", Arguments: `{"path":"a.go","start_line":1,"end_line":50}`}},
				}, nil
			case 2:
				return &llm.Response{
					ToolCalls: []llm.ToolCall{{ID: "call-2", Name: "read_file", Arguments: `{"path":"a.go","start_line":100,"end_line":150}`}},
				}, nil
			default:
				return &llm.Response{Content: `{"summary":"done"}`}, nil
			}
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			readExecutions++
			return "content", nil
		},
	}

	_, _, _, err := RunToolLoop(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readExecutions != 2 {
		t.Errorf("expected both distinct line ranges to execute, got %d executions", readExecutions)
	}
}
