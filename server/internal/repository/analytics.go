package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type AnalyticsRepo struct{ db *gorm.DB }

func NewAnalyticsRepo(db *gorm.DB) *AnalyticsRepo {
	return &AnalyticsRepo{db: db}
}

func (r *AnalyticsRepo) RecordLLMUsage(ctx context.Context, usage llm.UsageRecord) error {
	record := models.TokenUsage{
		Provider:         usage.Provider,
		Model:            usage.Model,
		LevelGroup:       usage.LevelGroup,
		PromptTokens:     usage.PromptTokens,
		OutputTokens:     usage.OutputTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		CostUSD:          usage.CostUSD,
		LatencyMS:        usage.LatencyMS,
		Status:           usage.Status,
		Error:            usage.Error,
	}
	if usage.OrgID != "" {
		record.OrgID = &usage.OrgID
	}
	if usage.CredentialID != "" {
		record.CredentialID = &usage.CredentialID
	}
	if usage.ProjectID != "" {
		record.ProjectID = &usage.ProjectID
	}
	if usage.AgentID != "" {
		record.AgentID = &usage.AgentID
	}
	if usage.TaskID != "" {
		record.TaskID = &usage.TaskID
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record llm usage: %w", err)
	}
	return nil
}

func (r *AnalyticsRepo) TokenUsage(ctx context.Context, orgID string, projectID string, since time.Time) ([]models.TokenUsageSummary, error) {
	query := r.db.WithContext(ctx).
		Table("token_usage").
		Select(`token_usage.project_id, token_usage.credential_id, COALESCE(pc.label, '') AS key_label, token_usage.provider, token_usage.model, token_usage.level_group,
			COUNT(*) AS requests,
			COUNT(CASE WHEN token_usage.status = 'ok' THEN 1 END) AS success_requests,
			COUNT(CASE WHEN token_usage.status != 'ok' THEN 1 END) AS failed_requests,
			COALESCE(SUM(token_usage.prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(token_usage.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(token_usage.prompt_tokens + token_usage.output_tokens), 0) AS total_tokens,
			COALESCE(SUM(token_usage.cost_usd), 0) AS cost_usd,
			COALESCE(AVG(token_usage.latency_ms), 0) AS avg_latency_ms`).
		Joins("LEFT JOIN provider_credentials pc ON token_usage.credential_id = pc.id").
		Group("token_usage.project_id, token_usage.credential_id, pc.label, token_usage.provider, token_usage.model, token_usage.level_group").
		Order("cost_usd DESC")

	if orgID != "" {
		query = query.Where("token_usage.org_id = ?", orgID)
	}
	if projectID != "" {
		query = query.Where("token_usage.project_id = ?", projectID)
	}
	if !since.IsZero() {
		query = query.Where("token_usage.created_at >= ?", since)
	}

	var summaries []models.TokenUsageSummary
	if err := query.Scan(&summaries).Error; err != nil {
		return nil, fmt.Errorf("aggregate token usage: %w", err)
	}
	return summaries, nil
}
