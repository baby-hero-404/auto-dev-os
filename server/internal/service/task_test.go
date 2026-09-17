package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newTaskServiceWithTaskRepoTestDB wires a real (sqlmock-backed) TaskRepo,
// needed for tests that exercise Clarify (GetByID + Update against the
// tasks table).
func newTaskServiceWithTaskRepoTestDB(t *testing.T) (*TaskService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		db.Close()
		t.Fatalf("failed to open gorm db: %v", err)
	}
	svc := NewTaskService(repository.NewTaskRepo(gormDB), repository.NewProjectRepo(gormDB), nil, repository.NewOrganizationRepo(gormDB))
	return svc, mock, func() { _ = db.Close() }
}

func TestTaskService_Clarify_ResumesPausedCLIStep(t *testing.T) {
	svc, mock, cleanup := newTaskServiceWithTaskRepoTestDB(t)
	defer cleanup()

	// Clarify calls s.repo.GetByID directly once, then s.repo.Update (which
	// internally calls GetByID again before issuing the UPDATE) — 2 SELECTs
	// total, in this exact order.
	selectRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "status", "spec_status", "paused_step"}).
			AddRow("task-1", models.TaskStatusCoding, models.TaskSpecStatusClarificationRequired, workflow.StepCLIImplement)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tasks" WHERE id = $1 ORDER BY "tasks"."id" LIMIT $2`)).
		WithArgs("task-1", 1).
		WillReturnRows(selectRows())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tasks" WHERE id = $1 ORDER BY "tasks"."id" LIMIT $2`)).
		WithArgs("task-1", 1).
		WillReturnRows(selectRows())
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "tasks" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := svc.Clarify(context.Background(), "task-1", models.ClarifyTaskInput{Context: "use option A"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != models.TaskStatusCoding {
		t.Errorf("expected resume status TaskStatusCoding (cli_implement's StatusOnResume), got %q", updated.Status)
	}
}

func TestTaskService_Clarify_AnalyzeFlowUnchanged(t *testing.T) {
	// Regression: a task with no PausedStep set (the legacy API-native
	// flow, which never sets this field) must resume exactly like today —
	// to TaskStatusAnalyzing.
	svc, mock, cleanup := newTaskServiceWithTaskRepoTestDB(t)
	defer cleanup()

	selectRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "status", "spec_status", "paused_step"}).
			AddRow("task-2", models.TaskStatusAnalyzing, models.TaskSpecStatusClarificationRequired, "")
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tasks" WHERE id = $1 ORDER BY "tasks"."id" LIMIT $2`)).
		WithArgs("task-2", 1).
		WillReturnRows(selectRows())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tasks" WHERE id = $1 ORDER BY "tasks"."id" LIMIT $2`)).
		WithArgs("task-2", 1).
		WillReturnRows(selectRows())
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "tasks" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := svc.Clarify(context.Background(), "task-2", models.ClarifyTaskInput{Context: "answer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != models.TaskStatusAnalyzing {
		t.Errorf("expected TaskStatusAnalyzing (unchanged legacy behavior), got %q", updated.Status)
	}
}

func TestBuildTaskAnalysis_Easy(t *testing.T) {
	task := &models.Task{
		Title:       "Fix typo in README",
		Description: "There is a typo on line 5 of the README. Change 'teh' to 'the'.",
		Complexity:  "",
	}
	analysis := buildTaskAnalysis(task)
	if analysis.Complexity != models.TaskComplexityEasy {
		t.Errorf("expected easy complexity, got %q", analysis.Complexity)
	}
	if len(analysis.ClarificationQuestions) > 0 {
		t.Errorf("expected no clarification questions for easy task with description")
	}
}

func TestBuildTaskAnalysis_Medium(t *testing.T) {
	task := &models.Task{
		Title:       "Implement new API endpoint for user profiles",
		Description: "Create a REST endpoint to fetch user profile data including avatar and bio fields.",
		Complexity:  "",
	}
	analysis := buildTaskAnalysis(task)
	if analysis.Complexity != models.TaskComplexityMedium {
		t.Errorf("expected medium complexity, got %q", analysis.Complexity)
	}
}

func TestBuildTaskAnalysis_Hard(t *testing.T) {
	task := &models.Task{
		Title:       "Implement RBAC permission system",
		Description: "Build a role-based access control system with hierarchical permissions and audit logging.",
		Complexity:  "",
	}
	analysis := buildTaskAnalysis(task)
	if analysis.Complexity != models.TaskComplexityHard {
		t.Errorf("expected hard complexity, got %q", analysis.Complexity)
	}
}

func TestBuildTaskAnalysis_ShortDescription(t *testing.T) {
	task := &models.Task{
		Title:       "Fix bug",
		Description: "Something is broken",
		Complexity:  "",
	}
	analysis := buildTaskAnalysis(task)
	if len(analysis.ClarificationQuestions) == 0 {
		t.Error("expected clarification questions for short description")
	}
}

func TestBuildTaskAnalysis_PresetComplexity(t *testing.T) {
	task := &models.Task{
		Title:       "Some task",
		Description: "A normal task with sufficient description text to avoid clarifications easily.",
		Complexity:  models.TaskComplexityHard,
	}
	analysis := buildTaskAnalysis(task)
	// The preset "hard" should be preserved since no signals override it.
	if analysis.Complexity != models.TaskComplexityHard {
		t.Errorf("expected hard complexity to be preserved, got %q", analysis.Complexity)
	}
}

func TestValidateTransition_Valid(t *testing.T) {
	tests := []struct {
		from, to string
	}{
		{models.TaskStatusTodo, models.TaskStatusAnalyzing},
		{models.TaskStatusAnalyzing, models.TaskStatusSpecReview},
		{models.TaskStatusSpecReview, models.TaskStatusCoding},
		{models.TaskStatusCoding, models.TaskStatusReviewing},
		{models.TaskStatusHumanReview, models.TaskStatusMerged},
	}
	for _, tc := range tests {
		if err := workflow.ValidateTaskTransition(tc.from, tc.to); err != nil {
			t.Errorf("transition %s→%s should be valid, got error: %v", tc.from, tc.to, err)
		}
	}
}

func TestValidateTransition_Invalid(t *testing.T) {
	tests := []struct {
		from, to string
	}{
		{models.TaskStatusTodo, models.TaskStatusMerged},
		{models.TaskStatusCoding, models.TaskStatusTodo},
		{models.TaskStatusMerged, models.TaskStatusCoding},
	}
	for _, tc := range tests {
		if err := workflow.ValidateTaskTransition(tc.from, tc.to); err == nil {
			t.Errorf("transition %s→%s should be invalid, but no error returned", tc.from, tc.to)
		}
	}
}

func TestValidateTransition_UnknownStatus(t *testing.T) {
	if err := workflow.ValidateTaskTransition("unknown_status", models.TaskStatusCoding); err == nil {
		t.Error("expected error for unknown current status")
	}
}

// TestTaskAnalysis_JSONRoundTrip verifies TaskAnalysis serialization.
func TestTaskAnalysis_JSONRoundTrip(t *testing.T) {
	original := models.TaskAnalysis{
		Complexity:             models.TaskComplexityMedium,
		Scope:                  "Test scope",
		AffectedFiles:          []models.AffectedFile{{File: "a.go"}, {File: "b.go"}},
		Risks:                  []string{"breaking change"},
		ExecutionPhases:        []models.ExecutionPhase{{Phase: "Phase 1", Tasks: []string{"step 1", "step 2"}}},
		ClarificationQuestions: []string{"what about X?"},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded models.TaskAnalysis
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Complexity != original.Complexity {
		t.Errorf("complexity mismatch: %q != %q", decoded.Complexity, original.Complexity)
	}
	if len(decoded.AffectedFiles) != len(original.AffectedFiles) {
		t.Errorf("affected_files length mismatch")
	}
}

// Ensure ErrValidation works as expected.
func TestErrValidation(t *testing.T) {
	err := ErrValidation("test error")
	if err.Error() != "validation: test error" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
	if !errors.Is(err, ErrInvalid) {
		t.Error("expected errors.Is(err, ErrInvalid) to be true")
	}
}

func isValidationErr(err error) bool {
	return errors.Is(err, ErrInvalid)
}
