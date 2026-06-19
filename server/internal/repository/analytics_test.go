package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAnalyticsRepo_TokenUsageFiltersOrgAndReturnsKeyLabel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	repo := NewAnalyticsRepo(gormDB)
	now := time.Date(2026, 6, 17, 8, 0, 0, 0, time.UTC)
	projectID := "project-1"
	credentialID := "cred-1"
	orgID := "org-1"

	rows := sqlmock.NewRows([]string{
		"project_id", "credential_id", "key_label", "provider", "model", "level_group",
		"requests", "success_requests", "failed_requests", "prompt_tokens", "output_tokens", "total_tokens", "cost_usd", "avg_latency_ms",
	}).AddRow(projectID, credentialID, "OpenAI-Prod-Key1", "openai", "gpt-4o", "balanced", 3, 2, 1, 120, 60, 180, 1.25, 42.5)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT token_usage.project_id, token_usage.credential_id, COALESCE(pc.label, '') AS key_label, token_usage.provider, token_usage.model, token_usage.level_group,
			COUNT(*) AS requests,
			COUNT(CASE WHEN token_usage.status = 'ok' THEN 1 END) AS success_requests,
			COUNT(CASE WHEN token_usage.status != 'ok' THEN 1 END) AS failed_requests,
			COALESCE(SUM(token_usage.prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(token_usage.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(token_usage.prompt_tokens + token_usage.output_tokens), 0) AS total_tokens,
			COALESCE(SUM(token_usage.cost_usd), 0) AS cost_usd,
			COALESCE(AVG(token_usage.latency_ms), 0) AS avg_latency_ms FROM "token_usage" LEFT JOIN provider_credentials pc ON token_usage.credential_id = pc.id WHERE token_usage.org_id = $1 AND token_usage.project_id = $2 AND token_usage.created_at >= $3 GROUP BY token_usage.project_id, token_usage.credential_id, pc.label, token_usage.provider, token_usage.model, token_usage.level_group ORDER BY cost_usd DESC`)).
		WithArgs(orgID, projectID, now).
		WillReturnRows(rows)

	summaries, err := repo.TokenUsage(context.Background(), orgID, projectID, now)
	if err != nil {
		t.Fatalf("token usage failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.KeyLabel != "OpenAI-Prod-Key1" || summary.CredentialID == nil || *summary.CredentialID != credentialID {
		t.Fatalf("unexpected summary metadata: %+v", summary)
	}
	if summary.Provider != "openai" || summary.Model != "gpt-4o" || summary.LevelGroup != "balanced" {
		t.Fatalf("unexpected summary fields: %+v", summary)
	}
}
