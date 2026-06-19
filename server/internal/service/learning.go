package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// LearningService manages the HITL suggestion lifecycle (create, review, apply).
type LearningService struct {
	suggestions *repository.LearningSuggestionRepo
	rules       *repository.RuleRepo
	skills      *SkillService
	promptRoot  string
}

func NewLearningService(suggestions *repository.LearningSuggestionRepo, rules *repository.RuleRepo) *LearningService {
	return &LearningService{suggestions: suggestions, rules: rules}
}

func (s *LearningService) SetSkillService(skills *SkillService) {
	s.skills = skills
}

func (s *LearningService) SetPromptRoot(promptRoot string) {
	s.promptRoot = filepath.Clean(promptRoot)
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
		return s.applyPromptPatchSuggestion(ctx, suggestion)
	case models.SuggestionTypeSkill:
		return s.applySkillSuggestion(ctx, suggestion)
	case models.SuggestionTypePattern:
		return s.applySkillSuggestion(ctx, suggestion)
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

func (s *LearningService) applySkillSuggestion(ctx context.Context, suggestion *models.LearningSuggestion) error {
	return fmt.Errorf("skill creation is no longer supported on the UI; please commit the skill to your Git repository registry instead")
}

func (s *LearningService) applyPromptPatchSuggestion(ctx context.Context, suggestion *models.LearningSuggestion) error {
	if s.promptRoot == "" {
		return fmt.Errorf("prompt root not configured")
	}

	patch, err := parsePromptPatch(suggestion.Content)
	if err != nil {
		return err
	}
	target, err := safeJoin(s.promptRoot, patch.Path)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read prompt patch target: %w", err)
	}
	currentText := string(current)
	if !strings.Contains(currentText, patch.Search) {
		return fmt.Errorf("apply prompt patch: search text not found in %s", patch.Path)
	}
	updated := strings.Replace(currentText, patch.Search, patch.Replace, 1)
	if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write prompt patch target: %w", err)
	}

	return s.markApplied(ctx, suggestion.ID, map[string]any{
		"patched_path": patch.Path,
		"applied_at":   time.Now(),
	})
}

func (s *LearningService) markApplied(ctx context.Context, suggestionID string, metadata map[string]any) error {
	if s.suggestions == nil {
		return fmt.Errorf("suggestion repository not configured")
	}
	applied := models.SuggestionStatusApplied
	metaJSON, _ := json.Marshal(metadata)
	feedback := string(metaJSON)
	_, err := s.suggestions.Update(ctx, suggestionID, models.UpdateSuggestionInput{
		Status:   &applied,
		Feedback: &feedback,
	})
	return err
}

type skillSuggestionContent struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

func skillInputFromSuggestion(suggestion *models.LearningSuggestion) models.CreateSkillInput {
	var parsed skillSuggestionContent
	if err := json.Unmarshal([]byte(suggestion.Content), &parsed); err == nil && parsed.Name != "" {
		if len(parsed.Schema) == 0 {
			parsed.Schema = json.RawMessage("{}")
		}
		return models.CreateSkillInput{
			Name:        parsed.Name,
			Description: parsed.Description,
			Schema:      parsed.Schema,
		}
	}

	schema, _ := json.Marshal(map[string]any{
		"source":          "learning_loop",
		"suggestion_id":   suggestion.ID,
		"suggestion_type": suggestion.SuggestionType,
		"content":         suggestion.Content,
	})
	return models.CreateSkillInput{
		Name:        slugSkillName(suggestion.Title),
		Description: firstNonEmpty(suggestion.Description, suggestion.Content),
		Schema:      schema,
	}
}

type promptPatch struct {
	Path    string `json:"path"`
	Search  string `json:"search"`
	Replace string `json:"replace"`
}

func parsePromptPatch(content string) (promptPatch, error) {
	var patch promptPatch
	if err := json.Unmarshal([]byte(content), &patch); err != nil {
		return patch, fmt.Errorf("prompt_patch content must be JSON with path, search, replace: %w", err)
	}
	if patch.Path == "" || patch.Search == "" {
		return patch, fmt.Errorf("prompt_patch requires path and search")
	}
	return patch, nil
}

func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("prompt_patch path must be relative")
	}
	target := filepath.Clean(filepath.Join(root, rel))
	rootWithSep := root + string(os.PathSeparator)
	if target != root && !strings.HasPrefix(target, rootWithSep) {
		return "", fmt.Errorf("prompt_patch path escapes prompt root")
	}
	return target, nil
}

func slugSkillName(title string) string {
	name := strings.ToLower(strings.TrimSpace(title))
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == '-' || r == '_' || r == ' ' {
			return '-'
		}
		return -1
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		return "learned-skill"
	}
	return "learned-" + name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
