package sandbox

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/google/uuid"
)

type fakeEmitter struct {
	mu     sync.Mutex
	events []models.TaskEvent
}

func (f *fakeEmitter) Emit(ctx context.Context, taskID, eventType string, schemaVersion int, payload json.RawMessage, artifactID *string) (*models.TaskEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := models.TaskEvent{TaskID: taskID, Type: eventType, SchemaVersion: schemaVersion, Payload: payload, ArtifactID: artifactID, SizeBytes: len(payload)}
	f.events = append(f.events, e)
	return &e, nil
}

func (f *fakeEmitter) last() models.TaskEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[len(f.events)-1]
}

type fakeArtifacts struct {
	mu      sync.Mutex
	created []models.WorkflowArtifact
}

func (f *fakeArtifacts) Create(ctx context.Context, artifact *models.WorkflowArtifact) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	artifact.ID = uuid.New().String()
	f.created = append(f.created, *artifact)
	return nil
}

func TestEventAdapter_ToolCall(t *testing.T) {
	emitter := &fakeEmitter{}
	a := NewEventAdapter(emitter, nil)

	err := a.ProcessLine(context.Background(), "task-1", `{"type":"tool_call","tool":"terminal","command":"go test"}`)
	if err != nil {
		t.Fatalf("ProcessLine: %v", err)
	}

	got := emitter.last()
	if got.Type != models.AgentEventToolStarted {
		t.Fatalf("expected %s, got %s", models.AgentEventToolStarted, got.Type)
	}
}

func TestEventAdapter_PlainTextFallback(t *testing.T) {
	emitter := &fakeEmitter{}
	a := NewEventAdapter(emitter, nil)

	err := a.ProcessLine(context.Background(), "task-1", "Analyzing authentication flow...")
	if err != nil {
		t.Fatalf("ProcessLine: %v", err)
	}

	got := emitter.last()
	if got.Type != models.AgentEventMessage {
		t.Fatalf("expected %s, got %s", models.AgentEventMessage, got.Type)
	}
}

func TestEventAdapter_MalformedJSONFallback(t *testing.T) {
	emitter := &fakeEmitter{}
	a := NewEventAdapter(emitter, nil)

	err := a.ProcessLine(context.Background(), "task-1", `{"type":"tool_call", not valid json`)
	if err != nil {
		t.Fatalf("ProcessLine should not error on malformed JSON: %v", err)
	}

	got := emitter.last()
	if got.Type != models.AgentEventMessage {
		t.Fatalf("expected fallback %s, got %s", models.AgentEventMessage, got.Type)
	}
}

func TestEventAdapter_CommandFinishedTruncatesTail(t *testing.T) {
	emitter := &fakeEmitter{}
	a := NewEventAdapter(emitter, nil)

	bigStdout := strings.Repeat("x", 20*1024)
	exitCode := 0
	line, _ := json.Marshal(map[string]any{
		"type":      "command_finished",
		"command":   "go build",
		"exit_code": exitCode,
		"stdout":    bigStdout,
	})

	if err := a.ProcessLine(context.Background(), "task-1", string(line)); err != nil {
		t.Fatalf("ProcessLine: %v", err)
	}

	got := emitter.last()
	var fields map[string]any
	if err := json.Unmarshal(got.Payload, &fields); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	tail, _ := fields["stdout_tail"].(string)
	if len(tail) > models.MaxTailBytes+64 {
		t.Fatalf("stdout_tail not truncated: %d bytes", len(tail))
	}
	if !strings.Contains(tail, "truncated") {
		t.Fatalf("expected truncation marker in stdout_tail")
	}
}

func TestEventAdapter_TestResultExternalizesLargeFailureLog(t *testing.T) {
	emitter := &fakeEmitter{}
	artifacts := &fakeArtifacts{}
	a := NewEventAdapter(emitter, artifacts)

	bigLog := strings.Repeat("FAIL\n", 8000) // > 40KB
	line, _ := json.Marshal(map[string]any{
		"type":   "test_result",
		"passed": 39,
		"failed": 3,
		"stdout": bigLog,
	})

	if err := a.ProcessLine(context.Background(), "task-1", string(line)); err != nil {
		t.Fatalf("ProcessLine: %v", err)
	}

	got := emitter.last()
	if got.ArtifactID == nil {
		t.Fatalf("expected non-nil ArtifactID for large failure log")
	}

	var fields map[string]any
	if err := json.Unmarshal(got.Payload, &fields); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := fields["summary"]; !ok {
		t.Fatalf("expected payload.summary to be set")
	}
	if len(got.Payload) > models.MaxPayloadBytes {
		t.Fatalf("payload not capped: %d bytes", len(got.Payload))
	}

	if len(artifacts.created) != 1 {
		t.Fatalf("expected 1 artifact created, got %d", len(artifacts.created))
	}
	var content map[string]any
	if err := json.Unmarshal(artifacts.created[0].Payload, &content); err != nil {
		t.Fatalf("unmarshal artifact payload: %v", err)
	}
	full, _ := content["content"].(string)
	if len(full) != len(bigLog) {
		t.Fatalf("artifact does not contain full log: got %d bytes, want %d", len(full), len(bigLog))
	}
}

func TestEventAdapter_SizeBytesMatchesPayload(t *testing.T) {
	emitter := &fakeEmitter{}
	a := NewEventAdapter(emitter, nil)

	if err := a.ProcessLine(context.Background(), "task-1", "short message"); err != nil {
		t.Fatalf("ProcessLine: %v", err)
	}

	got := emitter.last()
	if got.SizeBytes != len(got.Payload) {
		t.Fatalf("SizeBytes %d != len(payload) %d", got.SizeBytes, len(got.Payload))
	}
}
