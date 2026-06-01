package orchestrator

import (
	"context"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestPRGenerator_GenerateSummary(t *testing.T) {
	gen := NewPRGenerator()
	task := &models.Task{
		ID:          "test-task-id",
		Title:       "Add user validation",
		Description: "Implement email format validation in the user registration flow.",
		Complexity:  models.TaskComplexityEasy,
	}
	files := []string{"server/internal/service/auth.go", "server/internal/handler/auth.go"}

	summary := gen.GenerateSummary(context.Background(), task, files)

	if summary.Title != "[Auto Code OS] Add user validation" {
		t.Errorf("unexpected title: %s", summary.Title)
	}
	if summary.RiskLevel != models.PRRiskLow {
		t.Errorf("expected low risk for easy task with 2 files, got: %s", summary.RiskLevel)
	}
	if summary.Status != models.PRStatusOpen {
		t.Errorf("expected open status, got: %s", summary.Status)
	}
	if len(summary.ChangedFiles) != 2 {
		t.Errorf("expected 2 changed files, got: %d", len(summary.ChangedFiles))
	}
}

func TestPRGenerator_RiskAssessment_Migration(t *testing.T) {
	gen := NewPRGenerator()
	task := &models.Task{
		ID:         "test-task-migration",
		Title:      "Add audit logs table",
		Complexity: models.TaskComplexityMedium,
	}
	files := []string{"server/migration/000007_audit_logs.up.sql", "server/pkg/models/phase5.go"}

	summary := gen.GenerateSummary(context.Background(), task, files)

	if summary.RiskLevel != models.PRRiskHigh {
		t.Errorf("expected high risk for migration files, got: %s", summary.RiskLevel)
	}
}

func TestPRGenerator_RiskAssessment_HardWithMigration(t *testing.T) {
	gen := NewPRGenerator()
	task := &models.Task{
		ID:         "test-task-critical",
		Title:      "Refactor database schema",
		Complexity: models.TaskComplexityHard,
	}
	files := []string{"server/migration/000008_refactor.up.sql"}

	summary := gen.GenerateSummary(context.Background(), task, files)

	if summary.RiskLevel != models.PRRiskCritical {
		t.Errorf("expected critical risk for hard task with migration, got: %s", summary.RiskLevel)
	}
}

func TestPRGenerator_RiskAssessment_Config(t *testing.T) {
	gen := NewPRGenerator()
	task := &models.Task{
		ID:         "test-task-config",
		Title:      "Update config",
		Complexity: models.TaskComplexityEasy,
	}
	files := []string{"docker-compose.yml", "server/pkg/config/config.go"}

	summary := gen.GenerateSummary(context.Background(), task, files)

	if summary.RiskLevel != models.PRRiskMedium {
		t.Errorf("expected medium risk for config files, got: %s", summary.RiskLevel)
	}
}

func TestPRGenerator_RiskAssessment_ManyFiles(t *testing.T) {
	gen := NewPRGenerator()
	task := &models.Task{
		ID:         "test-many-files",
		Title:      "Large refactor",
		Complexity: models.TaskComplexityHard,
	}
	files := make([]string, 20)
	for i := range files {
		files[i] = "file" + string(rune('a'+i)) + ".go"
	}

	summary := gen.GenerateSummary(context.Background(), task, files)

	if summary.RiskLevel != models.PRRiskHigh {
		t.Errorf("expected high risk for hard task with many files, got: %s", summary.RiskLevel)
	}
}
