package service

import (
	"context"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type RuleService struct{ repo *repository.RuleRepo }

func NewRuleService(repo *repository.RuleRepo) *RuleService {
	return &RuleService{repo: repo}
}

func (s *RuleService) Create(ctx context.Context, projectID *string, input models.CreateRuleInput) (*models.Rule, error) {
	if strings.TrimSpace(input.Content) == "" {
		return nil, ErrValidation("content is required")
	}
	if projectID == nil || strings.TrimSpace(*projectID) == "" {
		return nil, ErrValidation("project id is required")
	}
	input.Scope = models.RuleScopeProject
	if input.Enforcement == "" {
		input.Enforcement = models.RuleEnforcementStrict
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *RuleService) CreateGlobal(ctx context.Context, orgID string, input models.CreateRuleInput) (*models.Rule, error) {
	if strings.TrimSpace(input.Content) == "" {
		return nil, ErrValidation("content is required")
	}
	if strings.TrimSpace(orgID) == "" {
		return nil, ErrValidation("organization id is required")
	}
	input.Scope = models.RuleScopeGlobal
	if input.Enforcement == "" {
		input.Enforcement = models.RuleEnforcementStrict
	}
	return s.repo.CreateGlobal(ctx, orgID, input)
}

func (s *RuleService) GetByID(ctx context.Context, id string) (*models.Rule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RuleService) ListByProjectID(ctx context.Context, projectID string) ([]models.Rule, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *RuleService) ListGlobalByOrgID(ctx context.Context, orgID string) ([]models.Rule, error) {
	return s.repo.ListGlobalByOrgID(ctx, orgID)
}

func (s *RuleService) Update(ctx context.Context, id string, input models.UpdateRuleInput) (*models.Rule, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *RuleService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func defaultRuleInputs(scope string) []models.CreateRuleInput {
	return []models.CreateRuleInput{
		{
			Scope:       scope,
			Content:     "Follow clean code principles: self-documenting code, meaningful variable names, small focused functions.",
			Enforcement: models.RuleEnforcementStrict,
		},
		{
			Scope:       scope,
			Content:     "All code changes must include tests. No PR may be merged without passing CI.",
			Enforcement: models.RuleEnforcementStrict,
		},
		{
			Scope:       scope,
			Content:     "Use conventional commit messages: feat:, fix:, docs:, refactor:, test:, chore:.",
			Enforcement: models.RuleEnforcementAdvisory,
		},
		{
			Scope:       scope,
			Content:     "Security first: never log secrets, validate all inputs, use parameterized queries.",
			Enforcement: models.RuleEnforcementStrict,
		},
		{
			Scope:       scope,
			Content:     "Document architectural decisions in ADRs. Update ARCHITECTURE.md when adding new packages or changing data flow.",
			Enforcement: models.RuleEnforcementAdvisory,
		},
		{
			Scope:       scope,
			Content:     "Strictly enforce the Socratic Gate (Definition of Ready): before starting implementation on any Medium/Hard tasks, ask the user at least 3 strategic questions to clarify specifications and boundary conditions. Do not start coding until requirements are explicitly confirmed.",
			Enforcement: models.RuleEnforcementStrict,
		},
		{
			Scope:       scope,
			Content:     "Ensure all code edits are surgical and targeted. Modify only the necessary parts of the codebase, preserving surrounding code style, docstrings, and comments.",
			Enforcement: models.RuleEnforcementStrict,
		},
		{
			Scope:       scope,
			Content:     "Practice Progressive Discovery and JIT Knowledge: read specific line ranges rather than loading entire files. Dynamically load/unload task-specific skills and remove them from context once the subtask is complete to avoid context window overflow.",
			Enforcement: models.RuleEnforcementStrict,
		},
		{
			Scope:       scope,
			Content:     "Always perform self-checks and verify your implementation by running local tests and linting before marking a task as complete.",
			Enforcement: models.RuleEnforcementStrict,
		},
	}
}

func (s *RuleService) SeedDefaultRules(ctx context.Context, projectID string) ([]models.Rule, error) {
	var created []models.Rule
	for _, input := range defaultRuleInputs(models.RuleScopeProject) {
		rule, err := s.repo.Create(ctx, &projectID, input)
		if err != nil {
			return nil, err
		}
		created = append(created, *rule)
	}
	return created, nil
}

func (s *RuleService) SeedGlobalDefaultRules(ctx context.Context, orgID string) ([]models.Rule, error) {
	var created []models.Rule
	for _, input := range defaultRuleInputs(models.RuleScopeGlobal) {
		rule, err := s.repo.CreateGlobal(ctx, orgID, input)
		if err != nil {
			return nil, err
		}
		created = append(created, *rule)
	}
	return created, nil
}
