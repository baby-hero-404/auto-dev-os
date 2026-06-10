package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCredentialPoolService_TestConnectionCallsProviderWithDecryptedKey(t *testing.T) {
	svc, mock, encryptedKey, cleanup := newCredentialPoolServiceForTest(t, "plain-key")
	defer cleanup()

	var testedCred models.ProviderCredential
	var testedKey string
	svc.withConnectionTester(func(_ context.Context, cred models.ProviderCredential, apiKey string) error {
		testedCred = cred
		testedKey = apiKey
		return nil
	})

	expectProviderCredentialByID(mock, "cred-1", "openai", encryptedKey)

	if err := svc.TestConnection(context.Background(), "cred-1"); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}
	if testedCred.ID != "cred-1" || testedCred.Provider != "openai" {
		t.Fatalf("tester received wrong credential: %+v", testedCred)
	}
	if testedKey != "plain-key" {
		t.Fatalf("tester received wrong api key: %q", testedKey)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestCredentialPoolService_TestConnectionReturnsProviderFailure(t *testing.T) {
	svc, mock, encryptedKey, cleanup := newCredentialPoolServiceForTest(t, "plain-key")
	defer cleanup()

	providerErr := errors.New("unauthorized")
	svc.withConnectionTester(func(context.Context, models.ProviderCredential, string) error {
		return providerErr
	})

	expectProviderCredentialByID(mock, "cred-1", "openai", encryptedKey)

	err := svc.TestConnection(context.Background(), "cred-1")
	if !errors.Is(err, providerErr) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func newCredentialPoolServiceForTest(t *testing.T, plainKey string) (*CredentialPoolService, sqlmock.Sqlmock, string, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		db.Close()
		t.Fatalf("open gorm db: %v", err)
	}
	cipher, err := NewSecretCipher("test-secret")
	if err != nil {
		db.Close()
		t.Fatalf("create cipher: %v", err)
	}
	encrypted, err := cipher.Encrypt(plainKey)
	if err != nil {
		db.Close()
		t.Fatalf("encrypt key: %v", err)
	}
	svc := NewCredentialPoolService(repository.NewProviderCredentialRepo(gormDB), cipher)
	return svc, mock, encrypted, func() { db.Close() }
}

func expectProviderCredentialByID(mock sqlmock.Sqlmock, id, provider, encryptedKey string) {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "org_id", "provider", "label", "encrypted_key", "base_url", "status", "priority", "metadata", "created_at", "updated_at",
	}).AddRow(id, "org-1", provider, "default", encryptedKey, "", models.ProviderCredentialStatusActive, 0, []byte("{}"), now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "provider_credentials" WHERE id = $1 ORDER BY "provider_credentials"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(rows)
}
