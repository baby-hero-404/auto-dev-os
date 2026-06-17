package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestSkillService_Create_EmptyName(t *testing.T) {
	svc := NewSkillService(nil, "")

	_, err := svc.Create(context.Background(), models.CreateSkillInput{Name: ""})
	if err == nil {
		t.Error("expected validation error for empty skill name")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestSkillService_Create_NilRepo(t *testing.T) {
	svc := NewSkillService(nil, "")

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when repo is nil and validation passes")
		}
	}()
	_, _ = svc.Create(context.Background(), models.CreateSkillInput{
		Name:        "plan-writing",
		Description: "Structured planning skill",
	})
}
