package sandbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// EventEmitter is the subset of TaskEventService that EventAdapter needs.
// Declared here (rather than importing internal/service directly) to avoid
// an import cycle: internal/service -> internal/orchestrator/steps ->
// internal/orchestrator/llmrunner -> internal/sandbox.
type EventEmitter interface {
	Emit(ctx context.Context, taskID, eventType string, schemaVersion int, payload json.RawMessage, artifactID *string) (*models.TaskEvent, error)
}

// ArtifactWriter is the subset of ArtifactRepo that EventAdapter needs.
type ArtifactWriter interface {
	Create(ctx context.Context, artifact *models.WorkflowArtifact) error
}

// EventAdapter normalizes raw CLI output lines into structured TaskEvents
// (docs/openspecs/status-driven-agent-workspace/design.md's Agent Adapter
// section), truncating/externalizing large payloads so task_events rows
// stay small and queryable.
type EventAdapter struct {
	events    EventEmitter
	artifacts ArtifactWriter
}

func NewEventAdapter(events EventEmitter, artifacts ArtifactWriter) *EventAdapter {
	return &EventAdapter{events: events, artifacts: artifacts}
}

// cliLine is the known JSON shape emitted by supported CLI engines. Lines
// that don't parse as this shape (or aren't JSON at all) fall back to a
// plain agent.message event.
type cliLine struct {
	Type      string `json:"type"`
	Tool      string `json:"tool"`
	Command   string `json:"command"`
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Summary   string `json:"summary"`
	ExitCode  *int   `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Passed    int    `json:"passed"`
	Failed    int    `json:"failed"`
	Text      string `json:"text"`
}

// ProcessLine parses a single line of CLI output and emits the corresponding
// TaskEvent. It never returns an error for malformed input — a parse
// failure degrades to an agent.message fallback rather than dropping the
// line or panicking.
func (a *EventAdapter) ProcessLine(ctx context.Context, taskID string, line string) error {
	var parsed cliLine
	if err := json.Unmarshal([]byte(line), &parsed); err != nil || parsed.Type == "" {
		return a.emitMessage(ctx, taskID, line)
	}

	switch parsed.Type {
	case "tool_call":
		return a.emitSimple(ctx, taskID, models.AgentEventToolStarted, map[string]any{
			"tool":    parsed.Tool,
			"command": parsed.Command,
		})
	case "tool_result":
		return a.emitSimple(ctx, taskID, models.AgentEventToolFinished, map[string]any{
			"tool": parsed.Tool,
		})
	case "reasoning":
		return a.emitSimple(ctx, taskID, models.AgentEventReasoningSummary, map[string]any{
			"summary": truncateTail(parsed.Summary),
		})
	case "file_changed":
		return a.emitSimple(ctx, taskID, models.AgentEventFileChanged, map[string]any{
			"path":      parsed.Path,
			"additions": parsed.Additions,
			"deletions": parsed.Deletions,
		})
	case "command_started":
		return a.emitSimple(ctx, taskID, models.AgentEventCommandStarted, map[string]any{
			"command": parsed.Command,
		})
	case "command_finished":
		return a.emitCommandFinished(ctx, taskID, parsed)
	case "test_result":
		return a.emitTestResult(ctx, taskID, parsed)
	default:
		return a.emitMessage(ctx, taskID, line)
	}
}

func (a *EventAdapter) emitMessage(ctx context.Context, taskID, text string) error {
	return a.emitSimple(ctx, taskID, models.AgentEventMessage, map[string]any{
		"text": truncateTail(text),
	})
}

func (a *EventAdapter) emitSimple(ctx context.Context, taskID, eventType string, fields map[string]any) error {
	payload, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	if len(payload) > models.MaxPayloadBytes {
		payload = mustMarshal(map[string]any{"summary": "payload truncated: exceeded size cap"})
	}
	_, err = a.events.Emit(ctx, taskID, eventType, 1, payload, nil)
	return err
}

// emitCommandFinished externalizes stdout/stderr to a workflow_artifact when
// the command failed and its combined output exceeds MaxTailBytes, keeping
// only a truncated tail + summary inline (see tasks.md 2.1).
func (a *EventAdapter) emitCommandFinished(ctx context.Context, taskID string, parsed cliLine) error {
	combined := parsed.Stdout + parsed.Stderr
	failed := parsed.ExitCode != nil && *parsed.ExitCode != 0

	fields := map[string]any{
		"command":     parsed.Command,
		"exit_code":   parsed.ExitCode,
		"stdout_tail": truncateTail(parsed.Stdout),
		"stderr_tail": truncateTail(parsed.Stderr),
	}

	var artifactID *string
	if failed && len(combined) > models.MaxTailBytes && a.artifacts != nil {
		id, err := a.writeArtifact(ctx, taskID, "event_output", combined)
		if err == nil {
			artifactID = id
			fields["summary"] = fmt.Sprintf("command failed (exit %d), %d bytes of output", derefInt(parsed.ExitCode), len(combined))
		}
	}

	return a.emitWithArtifact(ctx, taskID, models.AgentEventCommandFinished, fields, artifactID)
}

func (a *EventAdapter) emitTestResult(ctx context.Context, taskID string, parsed cliLine) error {
	fields := map[string]any{
		"passed": parsed.Passed,
		"failed": parsed.Failed,
	}

	var artifactID *string
	full := parsed.Stdout + parsed.Stderr
	if parsed.Failed > 0 && len(full) > models.MaxTailBytes && a.artifacts != nil {
		id, err := a.writeArtifact(ctx, taskID, "event_output", full)
		if err == nil {
			artifactID = id
			fields["summary"] = fmt.Sprintf("%d of %d tests failed", parsed.Failed, parsed.Passed+parsed.Failed)
		}
	}

	return a.emitWithArtifact(ctx, taskID, models.AgentEventTestResult, fields, artifactID)
}

func (a *EventAdapter) emitWithArtifact(ctx context.Context, taskID, eventType string, fields map[string]any, artifactID *string) error {
	payload, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	if len(payload) > models.MaxPayloadBytes {
		payload = mustMarshal(map[string]any{"summary": "payload truncated: exceeded size cap"})
	}
	_, err = a.events.Emit(ctx, taskID, eventType, 1, payload, artifactID)
	return err
}

func (a *EventAdapter) writeArtifact(ctx context.Context, taskID, artifactType, content string) (*string, error) {
	artifact := &models.WorkflowArtifact{
		TaskID:  taskID,
		Type:    artifactType,
		Payload: mustMarshal(map[string]any{"content": content}),
	}
	if err := a.artifacts.Create(ctx, artifact); err != nil {
		return nil, err
	}
	return &artifact.ID, nil
}

// EmitStatusChange is a convenience wrapper for status.changed events.
func (a *EventAdapter) EmitStatusChange(ctx context.Context, taskID, from, to string) error {
	return a.emitSimple(ctx, taskID, models.AgentEventStatusChanged, map[string]any{
		"from": from,
		"to":   to,
	})
}

// EmitError emits a task.error event — used both by CLI-output parsing and
// by the orchestrator's execution guardrails (tasks.md 2.3) when a guardrail
// trips and the task is about to transition to TaskStatusBlocked.
func (a *EventAdapter) EmitError(ctx context.Context, taskID, reason string, isRetryable bool) error {
	return a.emitSimple(ctx, taskID, models.AgentEventTaskError, map[string]any{
		"reason":       reason,
		"is_retryable": isRetryable,
	})
}

// truncateTail caps a string at models.MaxTailBytes, appending a marker
// noting the original size when truncation occurs.
func truncateTail(s string) string {
	if len(s) <= models.MaxTailBytes {
		return s
	}
	total := len(s)
	return s[:models.MaxTailBytes] + fmt.Sprintf("... truncated, %d bytes total", total)
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}
