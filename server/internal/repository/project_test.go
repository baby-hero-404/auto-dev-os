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

func newProjectRepoTest(t *testing.T) (*ProjectRepo, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		db.Close()
		t.Fatalf("failed to open gorm db: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
	}

	return NewProjectRepo(gormDB), mock, cleanup
}

func TestProjectRepo_ListByOrgID(t *testing.T) {
	repo, mock, cleanup := newProjectRepoTest(t)
	defer cleanup()

	ctx := context.Background()
	orgID := "org-uuid-123"
	projectID := "project-uuid-456"

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
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	p := projects[0]
	if p.ID != projectID {
		t.Fatalf("expected ID %q, got %q", projectID, p.ID)
	}
	if p.AgentsCount != 3 {
		t.Fatalf("expected AgentsCount 3, got %d", p.AgentsCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %s", err)
	}
}

func TestProjectRepo_CreatePersistsMaxReviewFixCycles(t *testing.T) {
	repo, mock, cleanup := newProjectRepoTest(t)
	defer cleanup()

	ctx := context.Background()
	orgID := "org-uuid-123"
	projectID := "project-uuid-456"
	input := models.CreateProjectInput{
		Name:               "Test Project",
		Description:        "A project for testing",
		DefaultModelLevel:  ptrString("balanced"),
		DefaultAutonomy:    ptrString("supervised"),
		AutoReviewPolicy:   ptrString("complexity_based"),
		MaxRetries:         ptrInt(5),
		MaxReviewFixCycles: ptrInt(7),
		DefaultBranch:      ptrString("main"),
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "projects" ("org_id","name","description","default_model_level","default_autonomy","auto_review_policy","max_retries","max_review_fix_cycles","default_branch","execution_engine","review_harness_policy","smart_routing","pipeline_config","created_at","updated_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,(NULL),$13,$14) RETURNING "id","cli_engine_config"`)).
		WithArgs(orgID, input.Name, input.Description, *input.DefaultModelLevel, *input.DefaultAutonomy, *input.AutoReviewPolicy, *input.MaxRetries, *input.MaxReviewFixCycles, *input.DefaultBranch, "api_native", "different_model", true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(projectID))
	mock.ExpectCommit()

	project, err := repo.Create(ctx, orgID, input)
	if err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	if project.MaxReviewFixCycles != 7 {
		t.Fatalf("expected MaxReviewFixCycles 7, got %d", project.MaxReviewFixCycles)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %s", err)
	}
}

func TestProjectRepo_UpdatePersistsMaxReviewFixCycles(t *testing.T) {
	repo, mock, cleanup := newProjectRepoTest(t)
	defer cleanup()

	ctx := context.Background()
	orgID := "org-uuid-123"
	projectID := "project-uuid-456"
	input := models.UpdateProjectInput{MaxReviewFixCycles: ptrInt(9)}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "projects" WHERE id = $1 ORDER BY "projects"."id" LIMIT $2`)).
		WithArgs(projectID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "org_id", "name", "description", "default_model_level", "default_autonomy", "auto_review_policy", "max_retries", "max_review_fix_cycles", "default_branch"}).
			AddRow(projectID, orgID, "Test Project", "A project for testing", "balanced", "supervised", "complexity_based", 3, 4, "main"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "projects" SET "max_review_fix_cycles"=$1,"updated_at"=$2 WHERE "id" = $3`)).
		WithArgs(9, sqlmock.AnyArg(), projectID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	project, err := repo.Update(ctx, projectID, input)
	if err != nil {
		t.Fatalf("update project failed: %v", err)
	}
	if project.MaxReviewFixCycles != 9 {
		t.Fatalf("expected returned project MaxReviewFixCycles to update to 9, got %d", project.MaxReviewFixCycles)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %s", err)
	}
}

func ptrString(v string) *string { return &v }

func ptrInt(v int) *int { return &v }
