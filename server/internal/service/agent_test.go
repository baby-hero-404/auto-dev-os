package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestAgentService_Create_EmptyName(t *testing.T) {
	svc := NewAgentService(nil)

	_, err := svc.Create(context.Background(), "proj-1", models.CreateAgentInput{Name: ""})
	if err == nil {
		t.Error("expected validation error for empty agent name")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestAgentService_Create_NilRepo(t *testing.T) {
	svc := NewAgentService(nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when repo is nil and validation passes")
		}
	}()
	_, _ = svc.Create(context.Background(), "proj-1", models.CreateAgentInput{
		Name: "backend-agent",
		Role: models.AgentRoleBackend,
		Goal: "Implement backend changes.",
	})
}

func TestAgentService_Create_RequiresRoleAndGoal(t *testing.T) {
	svc := NewAgentService(nil)

	_, err := svc.Create(context.Background(), "proj-1", models.CreateAgentInput{Name: "agent", Goal: "Do work"})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for missing role, got %v", err)
	}

	_, err = svc.Create(context.Background(), "proj-1", models.CreateAgentInput{Name: "agent", Role: models.AgentRoleBackend})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for missing goal, got %v", err)
	}
}
