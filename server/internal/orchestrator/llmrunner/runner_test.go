package llmrunner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TestCtxSleep_ReturnsWhenDurationElapses verifies the normal (non-canceled) path still waits
// out the full duration before returning nil.
func TestCtxSleep_ReturnsWhenDurationElapses(t *testing.T) {
	start := time.Now()
	if err := ctxSleep(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("expected to wait at least 20ms, only waited %v", elapsed)
	}
}

// TestCtxSleep_ReturnsImmediatelyOnCancellation verifies ctxSleep (Task 4.3 / REQ-M09) returns
// as soon as ctx is canceled instead of blocking for the full backoff duration like a plain
// time.Sleep would — the whole point of switching the outer retry backoff to be ctx-aware.
func TestCtxSleep_ReturnsImmediatelyOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := ctxSleep(ctx, 10*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a non-nil error when ctx is already canceled")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected ctxSleep to return promptly on cancellation, took %v", elapsed)
	}
}

type mockAgenticProvider struct {
	calls        int
	chatOptsUsed llm.ChatOptions
	messages     []llm.Message
}

func (m *mockAgenticProvider) Name() string { return "mock" }

func (m *mockAgenticProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return m.ChatWithOptions(ctx, messages, llm.ChatOptions{})
}

func (m *mockAgenticProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	m.calls++
	m.chatOptsUsed = opts
	m.messages = messages
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

func TestRunner_Run_IncludesNewFilePlaceholderInAffectedFiles(t *testing.T) {
	provider := &mockAgenticProvider{}

	runner := Runner{
		Provider: provider,
		Tools:    []llm.ToolDefinition{{Name: "search_replace"}},
		ToolExecutor: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return "ok", nil
		},
		ReadAffectedFileContent: func(ctx context.Context, task *models.Task, path string) (string, bool) {
			if strings.Contains(path, "existing.go") {
				return "package existing\n", true
			}
			return "", false // nonexistent file
		},
	}

	task := &models.Task{
		ID: "task-2",
		Analysis: []byte(`{
			"affected_files": [
				{"file": "existing.go"},
				{"file": "newfile.go"}
			]
		}`),
	}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	// We use "code_backend_0" as stepID which allows including affected files
	_, err := runner.Run(context.Background(), task, agent, "job-2", "code_backend_0", "implement the task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.messages) == 0 {
		t.Fatalf("expected messages to be captured")
	}

	var userPrompt string
	for _, m := range provider.messages {
		if m.Role == "user" && strings.Contains(m.Content, "Workflow step:") {
			userPrompt = m.Content
			break
		}
	}
	if userPrompt == "" {
		t.Fatalf("expected to find user prompt in captured messages")
	}

	if !strings.Contains(userPrompt, "--- existing.go ---") {
		t.Errorf("expected prompt to contain existing.go content header, got:\n%s", userPrompt)
	}
	if !strings.Contains(userPrompt, "package existing") {
		t.Errorf("expected prompt to contain existing.go content, got:\n%s", userPrompt)
	}
	if !strings.Contains(userPrompt, "--- newfile.go [NEW FILE — does not exist yet] ---") {
		t.Errorf("expected prompt to contain newfile.go placeholder, got:\n%s", userPrompt)
	}
	if !strings.Contains(userPrompt, "This file needs to be created. Use the create_file tool.") {
		t.Errorf("expected prompt to contain guidance for create_file tool, got:\n%s", userPrompt)
	}
}

type mockStateMachineProvider struct {
	calls []string
}

func (m *mockStateMachineProvider) Name() string { return "mock-sm" }

func (m *mockStateMachineProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return nil, nil
}

func (m *mockStateMachineProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	lastMsg := messages[len(messages)-1].Content
	if strings.Contains(lastMsg, "DISCOVERY") {
		m.calls = append(m.calls, "DISCOVERY")
		return &llm.Response{Model: "mock-model", Content: `{"summary":"discovery done"}`}, nil
	}
	if strings.Contains(lastMsg, "IMPLEMENTATION") {
		m.calls = append(m.calls, "IMPLEMENTATION")
		if len(m.calls) == 2 {
			return &llm.Response{
				Model:     "mock-model",
				ToolCalls: []llm.ToolCall{{ID: "call-imp", Name: "search_replace", Arguments: `{"path":"main.go"}`}},
			}, nil
		}
		return &llm.Response{Model: "mock-model", Content: `{"summary":"implementation done"}`}, nil
	}
	if strings.Contains(lastMsg, "VALIDATION") {
		m.calls = append(m.calls, "VALIDATION")
		if len(m.calls) == 4 {
			return &llm.Response{
				Model:     "mock-model",
				ToolCalls: []llm.ToolCall{{ID: "call-val", Name: "run_tests", Arguments: `{"command":"go test"}`}},
			}, nil
		}
		return &llm.Response{Model: "mock-model", Content: `{"summary":"validation done"}`}, nil
	}
	return &llm.Response{Model: "mock-model", Content: `{"summary":"done"}`}, nil
}

func TestRunner_Run_StateMachineMode(t *testing.T) {
	provider := &mockStateMachineProvider{}
	executedTools := make(map[string]bool)

	runner := Runner{
		Provider: provider,
		Tools: []llm.ToolDefinition{
			{Name: "search_replace"},
			{Name: "run_tests"},
			{Name: "read_file"},
		},
		ToolExecutor: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			executedTools[name] = true
			return "ok", nil
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level, message string) {},
		SaveArtifact: func(ctx context.Context, jobID, taskID, step, artType string, payload any) error { return nil },
	}

	task := &models.Task{
		ID: "task-1",
		Analysis: []byte(`{
			"execution_irs": [{
				"schema_version": "1.0",
				"node_id": "unit-1",
				"intent": {"capability": "BackendCode", "operation": "modify"},
				"constraints": [],
				"acceptance": [],
				"budget": {"discovery": 2, "implementation": 5, "validation": 2}
			}],
			"execution_units": [{
				"id": "unit-1",
				"objective": "BackendCode",
				"tasks": [],
				"execution_profile": {"agent": "backend", "skills": []},
				"constraints": {"parallelizable": false, "max_files": 1, "estimated_tokens": 1000, "max_risk": "low"}
			}],
			"execution_ir_targets": {
				"unit-1": ["main.go"]
			}
		}`),
	}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	ctx := context.WithValue(context.Background(), models.StateMachineEnabledCtxKey, true)

	out, err := runner.Run(ctx, task, agent, "job-1", "code_backend_0", "implement the task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.calls[0] != "DISCOVERY" {
		t.Errorf("expected first phase to be DISCOVERY, got %s", provider.calls[0])
	}
	if provider.calls[1] != "IMPLEMENTATION" {
		t.Errorf("expected second phase to be IMPLEMENTATION, got %s", provider.calls[1])
	}
	if provider.calls[3] != "VALIDATION" {
		t.Errorf("expected fourth phase to be VALIDATION, got %s", provider.calls[3])
	}

	if !executedTools["search_replace"] {
		t.Error("expected search_replace tool to be executed")
	}
	if !executedTools["run_tests"] {
		t.Error("expected run_tests tool to be executed")
	}

	parsed, ok := out["parsed"].(map[string]any)
	if !ok || parsed["summary"] != "validation done" {
		t.Errorf("expected parsed summary in output, got %v", out["parsed"])
	}
}
