package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestArtifactRepo(t *testing.T) {
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

	repo := NewArtifactRepo(gormDB)
	ctx := context.Background()

	jobID := "job-uuid-1"
	taskID := "task-uuid-2"
	artifactID := "artifact-uuid-3"

	t.Run("Create", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "workflow_artifacts"`)).
			WithArgs(jobID, taskID, "code", "patch", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(artifactID))
		mock.ExpectCommit()

		art := &models.WorkflowArtifact{
			JobID:   jobID,
			TaskID:  taskID,
			Step:    "code",
			Type:    "patch",
			Payload: json.RawMessage(`{}`),
		}
		err := repo.Create(ctx, art)
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
	})

	t.Run("ListByJobID", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "job_id", "task_id", "step", "type", "payload"}).
			AddRow(artifactID, jobID, taskID, "code", "patch", json.RawMessage(`{}`))

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "workflow_artifacts" WHERE job_id = $1`)).
			WithArgs(jobID).
			WillReturnRows(rows)

		arts, err := repo.ListByJobID(ctx, jobID)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if len(arts) != 1 {
			t.Errorf("expected 1, got %d", len(arts))
		}
	})
}
