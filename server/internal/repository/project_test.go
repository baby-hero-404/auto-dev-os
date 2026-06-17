package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestProjectRepo_ListByOrgID(t *testing.T) {
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

	repo := NewProjectRepo(gormDB)
	ctx := context.Background()

	orgID := "org-uuid-123"
	projectID := "project-uuid-456"

	t.Run("ListByOrgID", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "org_id", "name", "description", "repositories_count", "agents_count", "tasks_done_count", "tasks_total_count"}).
			AddRow(projectID, orgID, "Test Project", "A project for testing", 2, 3, 1, 5)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT projects.*, (SELECT COUNT(*) FROM repositories WHERE repositories.project_id = projects.id) AS repositories_count, (SELECT COUNT(*) FROM agents WHERE agents.org_id = projects.org_id AND (agents.assignment_strategy = 'auto_join' OR EXISTS (SELECT 1 FROM project_agents pa WHERE pa.agent_id = agents.id AND pa.project_id = projects.id))) AS agents_count, (SELECT COUNT(*) FROM tasks WHERE tasks.project_id = projects.id AND tasks.status IN ('done', 'completed', 'merged')) AS tasks_done_count, (SELECT COUNT(*) FROM tasks WHERE tasks.project_id = projects.id) AS tasks_total_count FROM "projects" WHERE projects.org_id = $1 ORDER BY projects.created_at DESC`)).
			WithArgs(orgID).
			WillReturnRows(rows)

		projects, err := repo.ListByOrgID(ctx, orgID)
		if err != nil {
			t.Fatalf("list by org ID failed: %v", err)
		}
		if len(projects) != 1 {
			t.Errorf("expected 1 project, got %d", len(projects))
		}
		p := projects[0]
		if p.ID != projectID {
			t.Errorf("expected ID %q, got %q", projectID, p.ID)
		}
		if p.AgentsCount != 3 {
			t.Errorf("expected AgentsCount 3, got %d", p.AgentsCount)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}
