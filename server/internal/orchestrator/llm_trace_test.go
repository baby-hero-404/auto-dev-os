package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestWriteLLMCallTrace_ChronologicalFormat(t *testing.T) {
	root := t.TempDir()
	orch := New(nil, nil, nil, nil, WithWorkspaceRoot(root))
	orch.initWkspace()

	ctx := context.Background()
	task := &models.Task{ID: "test-task-1"}
	agent := &models.Agent{ID: "agent-1", Name: "Test Agent", Role: "planner"}

	// Create a dummy workspace for the task
	ws := orch.wkspace.GetTaskWorkspace(task)
	_ = os.MkdirAll(ws.Root, 0o755)

	messages := []llm.Message{{Role: "user", Content: "Hello with secret: sk-123456789012345678901234567890123456789012345678"}}
	resp := &llm.Response{Model: "gpt-4", Content: "Response with sk-123456789012345678901234567890123456789012345678", PromptTokens: 10, OutputTokens: 20}
	parsed := map[string]any{"result": "success"}

	// 1. Write first trace (call-001-plan)
	orch.writeLLMCallTrace(ctx, task, agent, "plan", messages, resp, parsed)

	traceRoot := filepath.Join(ws.Root, "logs", "llm")

	// Verify directory exists
	dir1 := filepath.Join(traceRoot, "call-001-plan")
	if _, err := os.Stat(dir1); os.IsNotExist(err) {
		t.Fatalf("Expected directory %s to exist", dir1)
	}

	// Verify metadata
	metaData, err := os.ReadFile(filepath.Join(dir1, "metadata.json"))
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}
	if meta["step"] != "plan" {
		t.Errorf("Expected step 'plan', got %v", meta["step"])
	}
	if float64(meta["call_number"].(float64)) != 1 {
		t.Errorf("Expected call_number 1, got %v", meta["call_number"])
	}

	// Verify redaction
	promptData, _ := os.ReadFile(filepath.Join(dir1, "prompt.md"))
	if strings.Contains(string(promptData), "sk-123456789012345678901234567890123456789012345678") {
		t.Errorf("Secret was not redacted in prompt.md")
	}
	if !strings.Contains(string(promptData), "[REDACTED]") {
		t.Errorf("Expected [REDACTED] in prompt.md")
	}

	// 2. Write second trace (call-002-code)
	orch.writeLLMCallTrace(ctx, task, agent, "code", messages, resp, parsed)
	dir2 := filepath.Join(traceRoot, "call-002-code")
	if _, err := os.Stat(dir2); os.IsNotExist(err) {
		t.Fatalf("Expected directory %s to exist", dir2)
	}

	// 3. Write third trace (call-003-review) simulating loopback
	orch.writeLLMCallTrace(ctx, task, agent, "review", messages, resp, parsed)
	dir3 := filepath.Join(traceRoot, "call-003-review")
	if _, err := os.Stat(dir3); os.IsNotExist(err) {
		t.Fatalf("Expected directory %s to exist", dir3)
	}

	// 4. Verify global numbering continues
	orch.writeLLMCallTrace(ctx, task, agent, "fix", messages, resp, parsed)
	dir4 := filepath.Join(traceRoot, "call-004-fix")
	if _, err := os.Stat(dir4); os.IsNotExist(err) {
		t.Fatalf("Expected directory %s to exist", dir4)
	}

	// 5. Verify 5th trace (call-005-review) simulating second cycle
	orch.writeLLMCallTrace(ctx, task, agent, "review", messages, resp, parsed)
	dir5 := filepath.Join(traceRoot, "call-005-review")
	if _, err := os.Stat(dir5); os.IsNotExist(err) {
		t.Fatalf("Expected directory %s to exist", dir5)
	}
}
