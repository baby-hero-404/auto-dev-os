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
