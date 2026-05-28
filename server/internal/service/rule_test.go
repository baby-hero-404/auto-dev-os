package service

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestRuleService_Create_EmptyContent(t *testing.T) {
	svc := NewRuleService(nil)

	_, err := svc.Create(context.Background(), nil, models.CreateRuleInput{Content: ""})
	if err == nil {
		t.Error("expected validation error for empty rule content")
	}
	if !isValidationErr(err) {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestRuleService_Create_NilRepo(t *testing.T) {
	svc := NewRuleService(nil)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when repo is nil and validation passes")
		}
	}()
	projID := "proj-1"
	_, _ = svc.Create(context.Background(), &projID, models.CreateRuleInput{
		Content:     "Follow clean code principles.",
		Scope:       models.RuleScopeProject,
		Enforcement: models.RuleEnforcementStrict,
	})
}
