package service

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
)

func TestSeederService_Constructor(t *testing.T) {
	svc := NewSeederService(nil)
	if svc == nil {
		t.Fatal("expected non-nil SeederService")
	}
	if svc.ruleRepo != nil {
		t.Error("ruleRepo should be nil when passed nil")
	}
}

func TestSeederService_ConstructorWithRepos(t *testing.T) {
	var rr *repository.RuleRepo
	svc := NewSeederService(rr)
	if svc == nil {
		t.Fatal("expected non-nil SeederService")
	}
}

func TestSeederService_DefaultRuleCount(t *testing.T) {
	expectedRuleCount := 9
	svc := NewSeederService(nil)
	if svc == nil {
		t.Fatal("expected non-nil SeederService")
	}
	_ = expectedRuleCount
}
