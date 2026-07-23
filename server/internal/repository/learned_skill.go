package repository

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type LearnedSkillRepo struct {
	db *gorm.DB
}

func NewLearnedSkillRepo(db *gorm.DB) *LearnedSkillRepo {
	return &LearnedSkillRepo{db: db}
}

func (r *LearnedSkillRepo) Create(ctx context.Context, input models.CreateLearnedSkillInput) (*models.LearnedSkill, error) {
	skill := models.LearnedSkill{
		ProjectID:       input.ProjectID,
		Title:           input.Title,
		TriggerKeywords: input.TriggerKeywords,
		Content:         input.Content,
		Status:          input.Status,
		SourceTaskID:    input.SourceTaskID,
	}
	if skill.Status == "" {
		skill.Status = models.LearnedSkillStatusDraft
	}
	if err := r.db.WithContext(ctx).Create(&skill).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *LearnedSkillRepo) GetByID(ctx context.Context, id string) (*models.LearnedSkill, error) {
	var skill models.LearnedSkill
	if err := r.db.WithContext(ctx).First(&skill, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *LearnedSkillRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.LearnedSkill, error) {
	var skills []models.LearnedSkill
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&skills).Error; err != nil {
		return nil, err
	}
	return skills, nil
}

// SearchActiveByText runs a Postgres full-text-search match of query against
// each active skill's title + trigger_keywords, returning the top `limit`
// matches ordered by rank (REQ-002). Corpus per project is expected to be
// small (dozens of rows), so a simple inline to_tsvector is sufficient
// without a generated tsvector column/index.
func (r *LearnedSkillRepo) SearchActiveByText(ctx context.Context, projectID, query string, limit int) ([]models.LearnedSkill, error) {
	var skills []models.LearnedSkill
	err := r.db.WithContext(ctx).
		Raw(`SELECT * FROM learned_skills
			WHERE project_id = ? AND status = ?
			AND to_tsvector('english', title || ' ' || array_to_string(trigger_keywords, ' ')) @@ plainto_tsquery('english', ?)
			ORDER BY ts_rank(to_tsvector('english', title || ' ' || array_to_string(trigger_keywords, ' ')), plainto_tsquery('english', ?)) DESC
			LIMIT ?`, projectID, models.LearnedSkillStatusActive, query, query, limit).
		Scan(&skills).Error
	if err != nil {
		return nil, err
	}
	return skills, nil
}

func (r *LearnedSkillRepo) Update(ctx context.Context, id string, input models.UpdateLearnedSkillInput) (*models.LearnedSkill, error) {
	updates := map[string]any{}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.TriggerKeywords != nil {
		updates["trigger_keywords"] = pq.StringArray(input.TriggerKeywords)
	}
	if input.Content != nil {
		updates["content"] = *input.Content
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}
	if len(updates) > 0 {
		if err := r.db.WithContext(ctx).Model(&models.LearnedSkill{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return r.GetByID(ctx, id)
}

// IncrementUsage bumps usage_count (and success_count when success) for a
// batch of skill IDs at once (REQ-003), best-effort (caller should not fail
// the task lifecycle transition if this errors).
func (r *LearnedSkillRepo) IncrementUsage(ctx context.Context, ids []string, success bool) error {
	if len(ids) == 0 {
		return nil
	}
	query := r.db.WithContext(ctx).Model(&models.LearnedSkill{}).Where("id IN ?", ids)
	if success {
		return query.UpdateColumns(map[string]any{
			"usage_count":   gorm.Expr("usage_count + 1"),
			"success_count": gorm.Expr("success_count + 1"),
		}).Error
	}
	return query.UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}

func (r *LearnedSkillRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.LearnedSkill{}, "id = ?", id).Error
}
