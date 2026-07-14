package llmrunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/prompts"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

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

func TestRunner_Run_StateMachineMode_WritesTrace(t *testing.T) {
	var traceCalls int
	r := Runner{
		Provider: &mockStateMachineProvider{}, // Return a single Done-state JSON response, no tool calls
		Tools: []llm.ToolDefinition{
			{Name: "search_replace"},
		},
		ToolExecutor: func(ctx context.Context, name, args string) (string, error) {
			return "", nil
		},
		WriteTrace: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, msgs []llm.Message, resp *llm.Response, parsed map[string]any, iteration int, latency time.Duration) {
			traceCalls++
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level, message string) {},
	}
	ctx := context.WithValue(context.Background(), models.StateMachineEnabledCtxKey, true)
	task := &models.Task{ID: "t1"}
	agent := &models.Agent{ID: "a1"}

	_, err := r.Run(ctx, task, agent, "job1", "code_backend", "do the thing")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if traceCalls == 0 {
		t.Fatal("expected WriteTrace to be called at least once in state machine mode, got 0 calls")
	}
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

// mockSalvageProvider always completes DISCOVERY immediately, then keeps calling
// search_replace forever in IMPLEMENTATION so the node exhausts its implementation
// budget with edits applied, driving it to SALVAGED (never reaching VALIDATION).
type mockSalvageProvider struct{}

func (m *mockSalvageProvider) Name() string { return "mock-salvage" }

func (m *mockSalvageProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return nil, nil
}

func (m *mockSalvageProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	lastMsg := messages[len(messages)-1].Content
	if strings.Contains(lastMsg, "DISCOVERY") {
		return &llm.Response{Model: "mock-model", Content: `{"summary":"discovery done"}`}, nil
	}
	// IMPLEMENTATION: never signal completion, always emit another edit.
	return &llm.Response{
		Model:     "mock-model",
		ToolCalls: []llm.ToolCall{{ID: "call-imp", Name: "search_replace", Arguments: `{"path":"main.go"}`}},
	}, nil
}

// TestRunner_Run_StateMachineMode_SnapshotRoundTrip is the ExecutionSnapshot round-trip test
// required by docs/openspecs/execution-semantics-2026/tasks.md Task 3.1: a SALVAGED node's
// snapshot must (a) record how much of the implementation budget was actually consumed
// (Iteration), and (b) hash to the same PromptHash that an independent resume call
// (Runner.BuildInitialMessages, as used by llm_step.go) would reconstruct — otherwise the
// resume path in production can never recognize the snapshot as byte-identical (REQ-003).
func TestRunner_Run_StateMachineMode_SnapshotRoundTrip(t *testing.T) {
	provider := &mockSalvageProvider{}

	const wantDiff = "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n"

	var savedSnapshot *models.ExecutionSnapshot
	runner := Runner{
		Provider: provider,
		Tools: []llm.ToolDefinition{
			{Name: "search_replace"},
			{Name: "run_tests"},
			{Name: "read_file"},
		},
		ToolExecutor: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return "ok", nil
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level, message string) {},
		CaptureDiff: func(ctx context.Context, task *models.Task, agent *models.Agent, worktreeSuffix string) (string, error) {
			return wantDiff, nil
		},
		SaveArtifact: func(ctx context.Context, jobID, taskID, step, artType string, payload any) error {
			if artType == "execution_snapshot" {
				snap, ok := payload.(models.ExecutionSnapshot)
				if !ok {
					t.Fatalf("expected payload to be models.ExecutionSnapshot, got %T", payload)
				}
				savedSnapshot = &snap
			}
			return nil
		},
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
				"budget": {"discovery": 2, "implementation": 2, "validation": 2}
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
	stepID := "code_backend_0"
	instruction := "implement the task"

	ctx := context.WithValue(context.Background(), models.StateMachineEnabledCtxKey, true)

	out, err := runner.Run(ctx, task, agent, "job-1", stepID, instruction)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != "llm_partial" {
		t.Fatalf("expected llm_partial (SALVAGED) status, got %v", out["status"])
	}

	if savedSnapshot == nil {
		t.Fatal("expected an execution_snapshot artifact to be saved")
	}
	if savedSnapshot.CurrentState != "SALVAGED" {
		t.Errorf("expected CurrentState SALVAGED, got %s", savedSnapshot.CurrentState)
	}
	if savedSnapshot.Iteration != 2 {
		t.Errorf("expected Iteration to record the 2 consumed implementation iterations, got %d", savedSnapshot.Iteration)
	}
	if savedSnapshot.WorkspaceDiff != wantDiff {
		t.Errorf("expected WorkspaceDiff %q, got %q", wantDiff, savedSnapshot.WorkspaceDiff)
	}

	// Restore: independently reconstruct the initial prompt the way a resume would
	// (llm_step.go calls the same method) and confirm it hashes identically.
	restoredMessages, err := runner.BuildInitialMessages(ctx, task, agent, stepID, instruction)
	if err != nil {
		t.Fatalf("BuildInitialMessages failed: %v", err)
	}
	rawMsgs, _ := json.Marshal(restoredMessages)
	h := sha256.Sum256(rawMsgs)
	restoredHash := hex.EncodeToString(h[:])
	if restoredHash != savedSnapshot.PromptHash {
		t.Errorf("resume PromptHash mismatch: snapshot=%s restored=%s (byte-identical restore broken)", savedSnapshot.PromptHash, restoredHash)
	}

	// The remaining implementation budget the snapshot's Iteration exposes for a future
	// continue-with-remaining-budget resume.
	remaining := 2 - savedSnapshot.Iteration
	if remaining != 0 {
		t.Errorf("expected implementation budget fully exhausted at SALVAGED (remaining 0), got %d", remaining)
	}
}

// mockContractCaptureProvider completes every phase on the first turn (no tool calls) and
// records the full message list it saw on its very first call, so the test can inspect what
// the model was actually shown before any turn-specific mutation happened.
type mockContractCaptureProvider struct {
	firstCallMessages []llm.Message
}

func (m *mockContractCaptureProvider) Name() string { return "mock-contract" }

func (m *mockContractCaptureProvider) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return nil, nil
}

func (m *mockContractCaptureProvider) ChatWithOptions(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
	if m.firstCallMessages == nil {
		m.firstCallMessages = append([]llm.Message{}, messages...)
	}
	return &llm.Response{Model: "mock-model", Content: `{"summary":"done"}`}, nil
}

// TestRunner_Run_StateMachineMode_CompilesExecutionContract verifies the PromptCompiler wiring
// (design.md: State Machine -> Prompt Compiler -> LLM Node Executor): when a Compiler is set,
// runStateMachine must actually surface the node's constraints and acceptance criteria to the
// model. Previously PromptCompiler was fully built and tested but never called from any runtime
// path, so this information never reached the LLM at all.
func TestRunner_Run_StateMachineMode_CompilesExecutionContract(t *testing.T) {
	provider := &mockContractCaptureProvider{}

	runner := Runner{
		Provider: provider,
		Compiler: prompts.NewDefaultPromptCompiler("default"),
		Tools: []llm.ToolDefinition{
			{Name: "search_replace"},
			{Name: "read_file"},
		},
		ToolExecutor: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return "ok", nil
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level, message string) {},
	}

	task := &models.Task{
		ID: "task-1",
		Analysis: []byte(`{
			"execution_irs": [{
				"schema_version": "1.0",
				"node_id": "unit-1",
				"intent": {"capability": "BackendCode", "operation": "modify"},
				"constraints": ["must not modify existing tests"],
				"acceptance": ["all existing tests still pass"],
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

	if _, err := runner.Run(ctx, task, agent, "job-1", "code_backend_0", "implement the task"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.firstCallMessages) == 0 {
		t.Fatal("expected the LLM to have been called at least once")
	}
	var combined strings.Builder
	for _, m := range provider.firstCallMessages {
		combined.WriteString(m.Content)
		combined.WriteString("\n")
	}
	got := combined.String()

	for _, want := range []string{
		"=== Execution Contract ===",
		"must not modify existing tests",
		"all existing tests still pass",
		"main.go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected compiled execution contract to contain %q, got:\n%s", want, got)
		}
	}
}

// TestRunner_Run_StateMachineMode_FallbackIRPassesCompiler verifies that when no ExecutionIR
// exists at all (analysis has no execution_irs), the synthesized fallback IR in
// resolveExecutionIRForStep is still schema-valid — otherwise wiring a Compiler in would make
// Compile() error on every task lacking IR data, which is a common path (BuildExecutionIRs'
// fallback only covers ExecutionUnits, not every task shape).
func TestRunner_Run_StateMachineMode_FallbackIRPassesCompiler(t *testing.T) {
	provider := &mockContractCaptureProvider{}
	var loggedWarnings []string

	runner := Runner{
		Provider: provider,
		Compiler: prompts.NewDefaultPromptCompiler("default"),
		Tools:    []llm.ToolDefinition{{Name: "search_replace"}},
		ToolExecutor: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			return "ok", nil
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level, message string) {
			if level == "warn" {
				loggedWarnings = append(loggedWarnings, message)
			}
		},
	}

	task := &models.Task{ID: "task-1"} // no Analysis at all -> fallback IR path
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	ctx := context.WithValue(context.Background(), models.StateMachineEnabledCtxKey, true)

	if _, err := runner.Run(ctx, task, agent, "job-1", "code_backend_0", "implement the task"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, w := range loggedWarnings {
		if strings.Contains(w, "PromptCompiler failed") {
			t.Errorf("expected the fallback IR to pass Compile() without error, got warning: %s", w)
		}
	}

	var combined strings.Builder
	for _, m := range provider.firstCallMessages {
		combined.WriteString(m.Content)
	}
	if !strings.Contains(combined.String(), "=== Execution Contract ===") {
		t.Error("expected the execution contract to still be compiled and included for the fallback IR")
	}
}
