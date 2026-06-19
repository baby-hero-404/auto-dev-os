package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestProjectService_Create_EmptyName(t *testing.T) {
	// SeederService can be nil because validation should reject before reaching it.
	svc := NewProjectService(nil, &SeederService{}, "")

	_, err := svc.Create(context.Background(), "org-123", models.CreateProjectInput{Name: ""})
	if err == nil {
		t.Error("expected validation error for empty project name")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestProjectService_Constructor(t *testing.T) {
	seeder := &SeederService{}
	svc := NewProjectService(nil, seeder, "/tmp")
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.seeder != seeder {
		t.Error("seeder not properly assigned")
	}
	if svc.dataRoot != "/tmp" {
		t.Error("dataRoot not properly assigned")
	}
}
