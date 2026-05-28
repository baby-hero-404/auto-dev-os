package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestRepositoryService_Create_EmptyURL(t *testing.T) {
	svc := NewRepositoryService(nil)

	_, err := svc.Create(context.Background(), "proj-1", models.CreateRepositoryInput{URL: ""})
	if err == nil {
		t.Error("expected validation error for empty url")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestRepositoryService_ListRemoteRepos_EmptyToken(t *testing.T) {
	svc := NewRepositoryService(nil)

	_, err := svc.ListRemoteRepos(context.Background(), "")
	if err == nil {
		t.Error("expected validation error for empty token")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestRepositoryService_Constructor(t *testing.T) {
	svc := NewRepositoryService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.gitProvider == nil {
		t.Error("expected gitProvider to be initialized")
	}
	if svc.workspaceDir == "" {
		t.Error("expected workspaceDir to be set")
	}
}
