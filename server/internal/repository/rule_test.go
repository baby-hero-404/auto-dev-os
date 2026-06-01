package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRuleRepo_Lifecycle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	repo := NewRuleRepo(gormDB)
	ctx := context.Background()

	projectID := "test-project-1"
	ruleID := "rule-uuid-123"

	// 1. Create Test
	t.Run("Create", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "rules"`)).
			WithArgs(projectID, models.RuleScopeProject, "Must write tests.", models.RuleEnforcementStrict, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ruleID))
		mock.ExpectCommit()

		rule, err := repo.Create(ctx, &projectID, models.CreateRuleInput{
			Content:     "Must write tests.",
			Scope:       models.RuleScopeProject,
			Enforcement: models.RuleEnforcementStrict,
		})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if rule.ID != ruleID {
			t.Errorf("expected ID %q, got %q", ruleID, rule.ID)
		}
	})

	// 2. GetByID Test
	t.Run("GetByID", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "project_id", "scope", "content", "enforcement"}).
			AddRow(ruleID, &projectID, models.RuleScopeProject, "Must write tests.", models.RuleEnforcementStrict)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "rules" WHERE id = $1`)).
			WithArgs(ruleID, 1).
			WillReturnRows(rows)

		rule, err := repo.GetByID(ctx, ruleID)
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if rule.Content != "Must write tests." {
			t.Errorf("unexpected content: %s", rule.Content)
		}
	})

	// 3. ListByProjectID Test
	t.Run("ListByProjectID", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "project_id", "scope", "content", "enforcement"}).
			AddRow(ruleID, &projectID, models.RuleScopeProject, "Must write tests.", models.RuleEnforcementStrict)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "rules" WHERE project_id = $1 OR scope = 'global'`)).
			WithArgs(projectID).
			WillReturnRows(rows)

		rules, err := repo.ListByProjectID(ctx, projectID)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if len(rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(rules))
		}
	})

	// 4. Update Test
	t.Run("Update", func(t *testing.T) {
		// Mock GetByID inside Update
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "rules" WHERE id = $1`)).
			WithArgs(ruleID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "scope", "content", "enforcement"}).
				AddRow(ruleID, &projectID, models.RuleScopeProject, "Must write tests.", models.RuleEnforcementStrict))

		// Mock Updates query
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "rules"`)).
			WithArgs("Must write comprehensive tests.", sqlmock.AnyArg(), ruleID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		newContent := "Must write comprehensive tests."
		rule, err := repo.Update(ctx, ruleID, models.UpdateRuleInput{
			Content: &newContent,
		})
		if err != nil {
			t.Fatalf("update failed: %v", err)
		}
		if rule.Content != newContent {
			t.Errorf("expected content %q, got %q", newContent, rule.Content)
		}
	})

	// 5. Delete Test
	t.Run("Delete", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "rules" WHERE id = $1`)).
			WithArgs(ruleID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := repo.Delete(ctx, ruleID)
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
	})

	// 6. Delete NotFound Test
	t.Run("DeleteNotFound", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "rules" WHERE id = $1`)).
			WithArgs("non-existent").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		err := repo.Delete(ctx, "non-existent")
		if err == nil {
			t.Error("expected error deleting non-existent rule, got nil")
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}
