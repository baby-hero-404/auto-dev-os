package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestLearningService_CreateSuggestion_Validation(t *testing.T) {
	svc := NewLearningService(nil, nil)

	_, err := svc.CreateSuggestion(context.Background(), models.CreateSuggestionInput{
		AgentID:        "",
		Title:          "Valid suggestion",
		SuggestionType: "rule",
	})
	if err == nil {
		t.Error("expected validation error for empty agent ID")
	}

	_, err = svc.CreateSuggestion(context.Background(), models.CreateSuggestionInput{
		AgentID:        "agent-1",
		Title:          "",
		SuggestionType: "rule",
	})
	if err == nil {
		t.Error("expected validation error for empty title")
	}
}

func TestLearningService_SkillInputFromSuggestion_JSONContent(t *testing.T) {
	schema := json.RawMessage(`{"command":"run"}`)
	input := skillInputFromSuggestion(&models.LearningSuggestion{
		Title:          "Ignored",
		SuggestionType: models.SuggestionTypeSkill,
		Content:        `{"name":"retry-runner","description":"Retry failed runs","schema":{"command":"run"}}`,
	})

	if input.Name != "retry-runner" {
		t.Fatalf("expected parsed skill name, got %q", input.Name)
	}
	if input.Description != "Retry failed runs" {
		t.Fatalf("expected parsed description, got %q", input.Description)
	}
	if string(input.Schema) != string(schema) {
		t.Fatalf("expected parsed schema %s, got %s", schema, input.Schema)
	}
}

func TestLearningService_ParsePromptPatchAndSafeJoin(t *testing.T) {
	patch, err := parsePromptPatch(`{"path":"core/system_prompt.md","search":"old","replace":"new"}`)
	if err != nil {
		t.Fatalf("parse prompt patch: %v", err)
	}
	if patch.Path != "core/system_prompt.md" || patch.Search != "old" || patch.Replace != "new" {
		t.Fatalf("unexpected patch: %+v", patch)
	}

	root := t.TempDir()
	target, err := safeJoin(root, "core/system_prompt.md")
	if err != nil {
		t.Fatalf("safe join: %v", err)
	}
	expected := filepath.Join(root, "core", "system_prompt.md")
	if target != expected {
		t.Fatalf("expected target %q, got %q", expected, target)
	}
	if _, err := safeJoin(root, "../outside.md"); err == nil {
		t.Fatal("expected path escape to be rejected")
	}
}

func TestLearningService_ApplyPromptPatchSuggestion(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "core", "system_prompt.md")
	if err := os.WriteFile(target, []byte("before\nold text\nafter\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewLearningService(nil, nil)
	svc.SetPromptRoot(root)
	err := svc.applyPromptPatchSuggestion(context.Background(), &models.LearningSuggestion{
		ID:      "suggestion-1",
		Content: `{"path":"core/system_prompt.md","search":"old text","replace":"new text"}`,
	})
	if err == nil {
		t.Fatal("expected nil suggestion repo error after file patch")
	}
	updated, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(updated) != "before\nnew text\nafter\n" {
		t.Fatalf("unexpected patched content: %q", updated)
	}
}

func TestLearningService_ApproveSuggestion_NilRepo(t *testing.T) {
	svc := NewLearningService(nil, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic or error since suggestion repo is nil")
		}
	}()

	_, _ = svc.ApproveSuggestion(context.Background(), "suggestion-1", "user-1")
}
