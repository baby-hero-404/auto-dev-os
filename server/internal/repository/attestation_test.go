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

func newAttestationRepoTest(t *testing.T) (*AttestationRepo, *AttestationKeyRepo, sqlmock.Sqlmock, func()) {
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
	cleanup := func() { _ = db.Close() }
	return NewAttestationRepo(gormDB), NewAttestationKeyRepo(gormDB), mock, cleanup
}

func TestAttestationRepo_Create(t *testing.T) {
	repo, _, mock, cleanup := newAttestationRepoTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "attestations"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("att-1"))
	mock.ExpectCommit()

	a, err := repo.Create(context.Background(), models.CreateAttestationInput{
		TaskID:     "t1",
		CommitHash: "deadbeef",
		KeyID:      "key123",
		Envelope:   []byte(`{"payloadType":"x"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.CommitHash != "deadbeef" {
		t.Errorf("expected commit_hash deadbeef, got %q", a.CommitHash)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}

func TestAttestationRepo_GetByCommitHash_NotFound(t *testing.T) {
	repo, _, mock, cleanup := newAttestationRepoTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestations" WHERE commit_hash = $1`)).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.GetByCommitHash(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for missing commit hash, got nil")
	}
}

func TestAttestationKeyRepo_GetActive_NotFoundWhenNoneExist(t *testing.T) {
	_, keyRepo, mock, cleanup := newAttestationRepoTest(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestation_keys" WHERE status = $1`)).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"key_id"}))

	if _, err := keyRepo.GetActive(context.Background()); err == nil {
		t.Fatal("expected error when no active key exists, got nil")
	}
}

func TestAttestationKeyRepo_RetireActiveKeys(t *testing.T) {
	_, keyRepo, mock, cleanup := newAttestationRepoTest(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "attestation_keys" SET "status"=$1 WHERE status = $2`)).
		WithArgs("retired", "active").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := keyRepo.RetireActiveKeys(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}
