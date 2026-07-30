package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestOrganizationService_Create_EmptyName(t *testing.T) {
	svc := NewOrganizationService(nil)

	_, err := svc.Create(context.Background(), models.CreateOrganizationInput{Name: ""})
	if err == nil {
		t.Error("expected validation error for empty name")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestOrganizationService_Create_NilRepo(t *testing.T) {
	// When repo is nil but validation passes, we expect a nil-pointer panic.
	// This verifies that validation runs first and guards the repo call.
	svc := NewOrganizationService(nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when repo is nil and validation passes")
		}
	}()
	_, _ = svc.Create(context.Background(), models.CreateOrganizationInput{Name: "valid-org"})
}

func TestOrganizationService_Update_InvalidDefaultExecutionProviders(t *testing.T) {
	svc := NewOrganizationService(nil)

	_, err := svc.Update(context.Background(), "org-1", models.UpdateOrganizationInput{
		DefaultExecutionProviders: []byte(`[{"type":"bogus","ref":"x"}]`),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid default_execution_providers")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestOrganizationService_Update_EmptyDefaultExecutionProvidersIsNoop(t *testing.T) {
	// Validation must pass through an absent/empty field without reaching the
	// repo call with something it can't handle — nil repo panicking here
	// (same technique as TestOrganizationService_Create_NilRepo) proves
	// validation didn't reject it and let execution continue to s.repo.Update.
	svc := NewOrganizationService(nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic reaching the nil repo, meaning validation passed through")
		}
	}()
	_, _ = svc.Update(context.Background(), "org-1", models.UpdateOrganizationInput{})
}
