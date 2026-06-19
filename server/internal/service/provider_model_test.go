package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestProviderModelService_Create_ValidateLevelGroup(t *testing.T) {
	svc := NewProviderModelService(nil)

	_, err := svc.Create(context.Background(), "org-1", models.CreateProviderModelInput{
		Provider:   "openai",
		LevelGroup: "gpt-4o",
		ModelName:  "gpt-4o",
	})
	if !isValidationErr(err) {
		t.Fatalf("expected validation error for invalid level_group, got %v", err)
	}
}

func TestProviderModelService_Create_DefaultIsActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	svc := NewProviderModelService(repository.NewProviderModelRepo(gormDB))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "provider_models"`)).
		WithArgs("org-1", "openai", models.ModelLevelBalanced, "gpt-4o", 0, true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pm-1"))
	mock.ExpectCommit()

	model, err := svc.Create(context.Background(), "org-1", models.CreateProviderModelInput{
		Provider:   "OPENAI",
		LevelGroup: "BALANCED",
		ModelName:  " gpt-4o ",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !model.IsActive {
		t.Fatalf("expected default is_active true")
	}
	if model.Provider != "openai" || model.LevelGroup != models.ModelLevelBalanced || model.ModelName != "gpt-4o" {
		t.Fatalf("unexpected normalized model: %+v", model)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestProviderModelService_List_QueryFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	svc := NewProviderModelService(repository.NewProviderModelRepo(gormDB))
	orgID := "org-1"
	provider := "OPENAI"
	levelGroup := "BALANCED"

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "org_id", "provider", "level_group", "model_name", "priority", "is_active", "created_at", "updated_at"}).
		AddRow("pm-1", orgID, "openai", models.ModelLevelBalanced, "gpt-4o", 0, true, now, now)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "provider_models" WHERE org_id = $1 AND provider = $2 AND level_group = $3 ORDER BY priority ASC, created_at DESC`)).
		WithArgs(orgID, "openai", models.ModelLevelBalanced).
		WillReturnRows(rows)

	out, err := svc.ListByOrg(context.Background(), orgID, models.ProviderModelFilter{
		Provider:   &provider,
		LevelGroup: &levelGroup,
	})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(out) != 1 || out[0].Provider != "openai" || out[0].LevelGroup != models.ModelLevelBalanced {
		t.Fatalf("unexpected list output: %+v", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
