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

func TestAgentService_Create_ModelLevelGroupValidationAndDefaulting(t *testing.T) {
	svc := NewAgentService(nil)

	// Test invalid route
	_, err := svc.prepareCreateInput(context.Background(), models.CreateAgentInput{
		Name:            "agent",
		Role:            models.AgentRoleBackend,
		Goal:            "Do work",
		ModelLevelGroup: "gpt-4o",
	})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for invalid model level group, got %v", err)
	}

	// Test valid route
	prepared, err := svc.prepareCreateInput(context.Background(), models.CreateAgentInput{
		Name:            "agent",
		Role:            models.AgentRoleBackend,
		Goal:            "Do work",
		ModelLevelGroup: models.ModelLevelPowerful,
	})
	if err != nil {
		t.Fatalf("expected no error for valid model level group, got %v", err)
	}
	if prepared.ModelLevelGroup != models.ModelLevelPowerful {
		t.Errorf("expected ModelLevelGroup powerful, got %q", prepared.ModelLevelGroup)
	}

	// Test defaulting by roles
	roleDefaults := map[string]string{
		models.AgentRolePlanner:         models.ModelLevelPowerful,
		models.AgentRoleDBArchitect:     models.ModelLevelPowerful,
		models.AgentRoleBackend:         models.ModelLevelBalanced,
		models.AgentRoleFrontend:        models.ModelLevelBalanced,
		models.AgentRoleReviewer:        models.ModelLevelFast,
		models.AgentRoleSecurityAuditor: models.ModelLevelPowerful,
		models.AgentRoleQA:              models.ModelLevelBalanced,
	}

	for role, expectedRoute := range roleDefaults {
		prepared, err = svc.prepareCreateInput(context.Background(), models.CreateAgentInput{
			Name:            "agent",
			Role:            role,
			Goal:            "Do work",
			ModelLevelGroup: "",
		})
		if err != nil {
			t.Fatalf("prepare failed for role %s: %v", role, err)
		}
		if prepared.ModelLevelGroup != expectedRoute {
			t.Errorf("for role %q expected route %q, got %q", role, expectedRoute, prepared.ModelLevelGroup)
		}
	}
}

func TestAgentService_Create_RoleValidation(t *testing.T) {
	svc := NewAgentService(nil)

	// Test invalid role
	_, err := svc.Create(context.Background(), "proj-1", models.CreateAgentInput{
		Name: "agent",
		Role: "unknown-role",
		Goal: "Do work",
	})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for unknown role, got %v", err)
	}
}

func TestAgentService_Update_RejectsInvalidModelLevelGroup(t *testing.T) {
	svc := NewAgentService(nil)
	route := "gpt-4o"

	_, err := svc.Update(context.Background(), "agent-1", "org-1", models.UpdateAgentInput{ModelLevelGroup: &route})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for invalid model level group, got %v", err)
	}
}

func TestAgentService_Update_TrimsValidModelLevelGroup(t *testing.T) {
	svc := NewAgentService(nil)
	route := " powerful "

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when repo is nil and validation passes")
		}
		if route != models.ModelLevelPowerful {
			t.Fatalf("expected model level group to be trimmed to %q, got %q", models.ModelLevelPowerful, route)
		}
	}()
	_, _ = svc.Update(context.Background(), "agent-1", "org-1", models.UpdateAgentInput{ModelLevelGroup: &route})
}

func TestAgentService_Update_RejectsInvalidRole(t *testing.T) {
	svc := NewAgentService(nil)
	role := "unknown-role"

	_, err := svc.Update(context.Background(), "agent-1", "org-1", models.UpdateAgentInput{Role: &role})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for invalid role, got %v", err)
	}
}
