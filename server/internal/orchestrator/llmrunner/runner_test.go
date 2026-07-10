package llmrunner

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockAgenticProvider struct {
	calls        int
	chatOptsUsed llm.ChatOptions
}

func (m *mockAgenticProvider) Name() string { return "mock" }

func (m *mockAgenticProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (m *mockAgenticProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	m.calls++
	m.chatOptsUsed = opts
	if m.calls == 1 {
		return &llm.Response{
			Model:     "mock-model",
			ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "search_replace", Arguments: `{"path":"main.go"}`}},
		}, nil
	}
	return &llm.Response{Model: "mock-model", Content: `{"summary":"applied the fix"}`, PromptTokens: 10, OutputTokens: 5}, nil
}

func TestRunner_Run_AgenticModeUsesToolLoop(t *testing.T) {
	provider := &mockAgenticProvider{}
	var executedTool string

	runner := Runner{
		Provider: provider,
		Tools:    []llm.ToolDefinition{{Name: "search_replace"}},
		ToolExecutor: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			executedTool = name
			return "ok: applied", nil
		},
	}

	task := &models.Task{ID: "task-1"}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	out, err := runner.Run(context.Background(), task, agent, "job-1", "code_backend_0", "implement the change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.calls != 2 {
		t.Errorf("expected 2 provider calls (tool-call round + final), got %d", provider.calls)
	}
	if executedTool != "search_replace" {
		t.Errorf("expected search_replace tool to be executed, got %q", executedTool)
	}
	if len(provider.chatOptsUsed.Tools) != 1 {
		t.Errorf("expected tools to be passed to ChatWithOptions, got %+v", provider.chatOptsUsed)
	}
	parsed, ok := out["parsed"].(map[string]any)
	if !ok || parsed["summary"] != "applied the fix" {
		t.Errorf("expected parsed summary in output, got %v", out["parsed"])
	}
}

func TestRunner_ValidateSchema_AgenticRequiresSummaryNotPatch(t *testing.T) {
	r := Runner{}

	// Agentic mode: a patch/diff field is no longer required, but summary is.
	if err := r.validateSchema("code_backend_0", map[string]any{"summary": "did the thing"}, true); err != nil {
		t.Errorf("expected no error when summary is present in agentic mode, got %v", err)
	}
	if err := r.validateSchema("code_backend_0", map[string]any{}, true); err == nil {
		t.Error("expected an error when summary is missing in agentic mode")
	}

	// Non-agentic mode: still requires patch/diff, summary alone is not enough.
	if err := r.validateSchema("code_backend_0", map[string]any{"summary": "did the thing"}, false); err == nil {
		t.Error("expected an error when patch/diff is missing in non-agentic mode")
	}
	if err := r.validateSchema("code_backend_0", map[string]any{"patch": "diff --git a b"}, false); err != nil {
		t.Errorf("expected no error when patch is present in non-agentic mode, got %v", err)
	}
}
