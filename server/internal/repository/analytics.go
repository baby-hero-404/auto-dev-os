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
		Provider:     usage.Provider,
		Model:        usage.Model,
		Tier:         usage.Tier,
		PromptTokens: usage.PromptTokens,
		OutputTokens: usage.OutputTokens,
		CostUSD:      usage.CostUSD,
		LatencyMS:    usage.LatencyMS,
		Status:       usage.Status,
		Error:        usage.Error,
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

func (r *AnalyticsRepo) TokenUsage(ctx context.Context, projectID string, since time.Time) ([]models.TokenUsageSummary, error) {
	query := r.db.WithContext(ctx).
		Table("token_usage").
		Select(`project_id, provider, model, tier,
			COUNT(*) AS requests,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(prompt_tokens + output_tokens), 0) AS total_tokens,
			COALESCE(SUM(cost_usd), 0) AS cost_usd,
			COALESCE(AVG(latency_ms), 0) AS avg_latency_ms`).
		Group("project_id, provider, model, tier").
		Order("cost_usd DESC")

	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}

	var summaries []models.TokenUsageSummary
	if err := query.Scan(&summaries).Error; err != nil {
		return nil, fmt.Errorf("aggregate token usage: %w", err)
	}
	return summaries, nil
}
