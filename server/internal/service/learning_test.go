package service

import (
	"context"
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

func TestLearningService_ApproveSuggestion_NilRepo(t *testing.T) {
	svc := NewLearningService(nil, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic or error since suggestion repo is nil")
		}
	}()

	_, _ = svc.ApproveSuggestion(context.Background(), "suggestion-1", "user-1")
}
