package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// LearningService manages the HITL suggestion lifecycle (create, review, apply).
type LearningService struct {
	suggestions *repository.LearningSuggestionRepo
	rules       *repository.RuleRepo
}

func NewLearningService(suggestions *repository.LearningSuggestionRepo, rules *repository.RuleRepo) *LearningService {
	return &LearningService{suggestions: suggestions, rules: rules}
}

// CreateSuggestion proposes a new learning suggestion for human review.
func (s *LearningService) CreateSuggestion(ctx context.Context, input models.CreateSuggestionInput) (*models.LearningSuggestion, error) {
	if input.Title == "" {
		return nil, ErrValidation("suggestion title is required")
	}
	if input.AgentID == "" {
		return nil, ErrValidation("agent_id is required")
	}

	suggestion := &models.LearningSuggestion{
		AgentID:        input.AgentID,
		ProjectID:      input.ProjectID,
		TaskID:         input.TaskID,
		SuggestionType: input.SuggestionType,
		Title:          input.Title,
		Description:    input.Description,
		Content:        input.Content,
		Confidence:     input.Confidence,
		Status:         models.SuggestionStatusPending,
		Metadata:       json.RawMessage("{}"),
	}

	if err := s.suggestions.Create(ctx, suggestion); err != nil {
		return nil, err
	}

	slog.Info("learning: suggestion created",
		"id", suggestion.ID,
		"type", suggestion.SuggestionType,
		"confidence", suggestion.Confidence,
	)
	return suggestion, nil
}

// ListSuggestions returns suggestions filtered by agent and/or status.
func (s *LearningService) ListSuggestions(ctx context.Context, agentID, status string, limit int) ([]models.LearningSuggestion, error) {
	return s.suggestions.List(ctx, agentID, status, limit)
}

// GetSuggestion returns a single suggestion by ID.
func (s *LearningService) GetSuggestion(ctx context.Context, id string) (*models.LearningSuggestion, error) {
	return s.suggestions.GetByID(ctx, id)
}

// ApproveSuggestion marks a suggestion as approved and applies it if applicable.
func (s *LearningService) ApproveSuggestion(ctx context.Context, id, userID string) (*models.LearningSuggestion, error) {
	suggestion, err := s.suggestions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if suggestion.Status != models.SuggestionStatusPending {
		return nil, ErrValidation(fmt.Sprintf("can only approve pending suggestions (current: %s)", suggestion.Status))
	}

	status := models.SuggestionStatusApproved
	updated, err := s.suggestions.Update(ctx, id, models.UpdateSuggestionInput{
		Status:     &status,
		ReviewedBy: &userID,
	})
	if err != nil {
		return nil, err
	}

	// Auto-apply if it's a rule suggestion
	if err := s.applySuggestion(ctx, updated); err != nil {
		slog.Warn("learning: auto-apply failed after approval", "id", id, "error", err)
		// Don't fail the approval — the suggestion is still marked approved
	}

	slog.Info("learning: suggestion approved", "id", id, "type", suggestion.SuggestionType, "reviewer", userID)
	return updated, nil
}

// RejectSuggestion marks a suggestion as rejected with optional feedback.
func (s *LearningService) RejectSuggestion(ctx context.Context, id, userID, feedback string) (*models.LearningSuggestion, error) {
	suggestion, err := s.suggestions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if suggestion.Status != models.SuggestionStatusPending {
		return nil, ErrValidation(fmt.Sprintf("can only reject pending suggestions (current: %s)", suggestion.Status))
	}

	status := models.SuggestionStatusRejected
	updated, err := s.suggestions.Update(ctx, id, models.UpdateSuggestionInput{
		Status:     &status,
		ReviewedBy: &userID,
		Feedback:   &feedback,
	})
	if err != nil {
		return nil, err
	}

	slog.Info("learning: suggestion rejected", "id", id, "type", suggestion.SuggestionType, "reviewer", userID)
	return updated, nil
}

// applySuggestion executes the suggestion action (create rule, etc.).
func (s *LearningService) applySuggestion(ctx context.Context, suggestion *models.LearningSuggestion) error {
	switch suggestion.SuggestionType {
	case models.SuggestionTypeRule:
		return s.applyRuleSuggestion(ctx, suggestion)
	case models.SuggestionTypePromptPatch:
		// Prompt patches are logged for manual application — no auto-apply yet
		slog.Info("learning: prompt patch approved — manual application required", "id", suggestion.ID)
		return nil
	case models.SuggestionTypeSkill:
		// Skills are logged for manual registration
		slog.Info("learning: skill suggestion approved — manual registration required", "id", suggestion.ID)
		return nil
	case models.SuggestionTypePattern:
		// Patterns are informational
		slog.Info("learning: pattern approved — stored for reference", "id", suggestion.ID)
		return nil
	default:
		return nil
	}
}

// applyRuleSuggestion creates a new project rule from the suggestion content.
func (s *LearningService) applyRuleSuggestion(ctx context.Context, suggestion *models.LearningSuggestion) error {
	if s.rules == nil {
		return fmt.Errorf("rule repository not configured")
	}

	scope := models.RuleScopeProject
	if suggestion.ProjectID == nil {
		scope = models.RuleScopeGlobal
	}
	enforcement := models.RuleEnforcementAdvisory // AI-suggested rules start as advisory
	ruleInput := models.CreateRuleInput{
		Scope:       scope,
		Content:     suggestion.Content,
		Enforcement: enforcement,
	}

	rule, err := s.rules.Create(ctx, suggestion.ProjectID, ruleInput)
	if err != nil {
		return fmt.Errorf("apply rule suggestion: %w", err)
	}

	// Mark suggestion as applied
	applied := models.SuggestionStatusApplied
	now := time.Now()
	meta := map[string]any{"applied_rule_id": rule.ID, "applied_at": now}
	metaJSON, _ := json.Marshal(meta)
	feedback := string(metaJSON)
	_, _ = s.suggestions.Update(ctx, suggestion.ID, models.UpdateSuggestionInput{
		Status:   &applied,
		Feedback: &feedback,
	})

	slog.Info("learning: rule applied from suggestion",
		"suggestion_id", suggestion.ID,
		"rule_id", rule.ID,
	)
	return nil
}
