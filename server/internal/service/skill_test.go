package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func runGitCmd(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v, output: %s", args, err, string(out))
	}
}

func TestSkillService_GitSync(t *testing.T) {
	// 1. Create a dummy git repo on local filesystem
	gitSourceDir := t.TempDir()
	runGitCmd(t, gitSourceDir, "init")
	runGitCmd(t, gitSourceDir, "config", "user.email", "test@example.com")
	runGitCmd(t, gitSourceDir, "config", "user.name", "Test User")

	manifest := `{
		"skills": {
			"process": [
				{
					"id": "git-test-skill",
					"name": "Git Test Skill",
					"description": "Imported from local git",
					"path": "skills/git-test-skill.md"
				}
			]
		}
	}`
	err := os.WriteFile(filepath.Join(gitSourceDir, "registry.json"), []byte(manifest), 0644)
	if err != nil {
		t.Fatalf("failed to write registry.json: %v", err)
	}

	runGitCmd(t, gitSourceDir, "add", "registry.json")
	runGitCmd(t, gitSourceDir, "commit", "-m", "initial commit")

	// 2. Set up DB mock
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

	sourceRepo := repository.NewSkillSourceRepo(gormDB)
	tempDir := t.TempDir()
	svc := NewSkillService(nil, sourceRepo, tempDir)

	sourceID := "source-uuid-123"
	sourceURL := "file://" + filepath.ToSlash(gitSourceDir)

	// Mock GetByID for original lookup
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" WHERE id = $1 ORDER BY "skill_sources"."id" LIMIT $2`)).
		WithArgs(sourceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "status", "error"}).
			AddRow(sourceID, sourceURL, "pending", ""))

	// Inside Update(status = "syncing"): first GetByID is called
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" WHERE id = $1 ORDER BY "skill_sources"."id" LIMIT $2`)).
		WithArgs(sourceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "status", "error"}).
			AddRow(sourceID, sourceURL, "pending", ""))

	// Mock Update to status = "syncing"
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "skill_sources"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Inside Update(status = "synced"): first GetByID is called
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" WHERE id = $1 ORDER BY "skill_sources"."id" LIMIT $2`)).
		WithArgs(sourceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url", "status", "error"}).
			AddRow(sourceID, sourceURL, "syncing", ""))

	// Mock Update to status = "synced"
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "skill_sources"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Execute sync
	syncedSource, err := svc.SyncSource(context.Background(), sourceID)
	if err != nil {
		t.Fatalf("SyncSource failed: %v", err)
	}

	if syncedSource.Status != "synced" {
		t.Errorf("expected synced status, got '%s'", syncedSource.Status)
	}

	// 3. Verify registry
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" ORDER BY created_at ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).
			AddRow(sourceID, sourceURL))

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list skills: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(list))
	}
	if list[0].ID != "git-test-skill" {
		t.Errorf("expected skill ID 'git-test-skill', got '%s'", list[0].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSkillService_ListFiles_And_GetContent(t *testing.T) {
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, "git", "test-repo")
	if err := os.MkdirAll(filepath.Join(gitDir, "skills/core"), 0755); err != nil {
		t.Fatal(err)
	}

	file1 := filepath.Join(gitDir, "skills/core/SKILL.md")
	if err := os.WriteFile(file1, []byte("Markdown Content"), 0644); err != nil {
		t.Fatal(err)
	}

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

	sourceRepo := repository.NewSkillSourceRepo(gormDB)
	svc := NewSkillService(nil, sourceRepo, tempDir)

	sourceID := "source-uuid-123"
	sourceURL := "https://github.com/org/test-repo.git"

	// Mock GetByID for ListFiles
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" WHERE id = $1 ORDER BY "skill_sources"."id" LIMIT $2`)).
		WithArgs(sourceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(sourceID, sourceURL))

	files, err := svc.ListFiles(context.Background(), sourceID, "skills/core")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "SKILL.md" {
		t.Errorf("expected SKILL.md, got %s", files[0].Name)
	}

	// Mock GetByID for GetFileContent
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" WHERE id = $1 ORDER BY "skill_sources"."id" LIMIT $2`)).
		WithArgs(sourceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(sourceID, sourceURL))

	content, err := svc.GetFileContent(context.Background(), sourceID, "skills/core/SKILL.md")
	if err != nil {
		t.Fatalf("GetFileContent failed: %v", err)
	}
	if content.Content != "Markdown Content" {
		t.Errorf("expected 'Markdown Content', got '%s'", content.Content)
	}

	// Test boundary escape
	// Mock GetByID for escapes boundary test
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "skill_sources" WHERE id = $1 ORDER BY "skill_sources"."id" LIMIT $2`)).
		WithArgs(sourceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "url"}).AddRow(sourceID, sourceURL))

	_, err = svc.GetFileContent(context.Background(), sourceID, "../../outside.txt")
	if err == nil || !strings.Contains(err.Error(), "escapes boundary") {
		t.Errorf("expected boundary escape error, got %v", err)
	}
}

func TestSkillService_AddSource_ValidatesRepoURL(t *testing.T) {
	svc := NewSkillService(nil, nil, t.TempDir())

	_, err := svc.AddSource(context.Background(), models.CreateSkillSourceInput{URL: "not-a-repo"})
	if err == nil {
		t.Fatal("expected invalid repository URL error")
	}
	if !strings.Contains(err.Error(), "invalid repository URL") {
		t.Fatalf("expected invalid repository URL error, got %v", err)
	}
}
