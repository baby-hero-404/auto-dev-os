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

type fakeProviderModelSeeder struct {
	listCalls   int
	createCalls []models.CreateProviderModelInput
}

func (s *fakeProviderModelSeeder) ListByOrg(_ context.Context, _ string, _ models.ProviderModelFilter) ([]models.ProviderModel, error) {
	s.listCalls++
	if s.listCalls == 1 {
		return nil, nil
	}
	return []models.ProviderModel{{ID: "pm-1", Provider: "openai", LevelGroup: models.ModelLevelBalanced, ModelName: "gpt-4o", IsActive: true}}, nil
}

func (s *fakeProviderModelSeeder) Create(_ context.Context, _ string, input models.CreateProviderModelInput) (*models.ProviderModel, error) {
	s.createCalls = append(s.createCalls, input)
	return &models.ProviderModel{ID: "pm-seeded", Provider: input.Provider, LevelGroup: input.LevelGroup, ModelName: input.ModelName, Priority: input.Priority, IsActive: input.IsActive == nil || *input.IsActive}, nil
}

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

	expectProviderCredentialByIDAndOrg(mock, "cred-1", "org-1", "openai", encryptedKey)

	if err := svc.TestConnection(context.Background(), "org-1", "cred-1"); err != nil {
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

	expectProviderCredentialByIDAndOrg(mock, "cred-1", "org-1", "openai", encryptedKey)

	err := svc.TestConnection(context.Background(), "org-1", "cred-1")
	if !errors.Is(err, providerErr) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestCredentialPoolService_Create_AutoSeed_Idempotency(t *testing.T) {
	svc, mock, encryptedKey, cleanup := newCredentialPoolServiceForTest(t, "plain-key")
	defer cleanup()

	seeder := &fakeProviderModelSeeder{}
	svc.WithProviderModelSeeder(seeder)

	expectCreateCredential(mock, "cred-1", "openai", encryptedKey)
	expectCreateCredential(mock, "cred-2", "openai", encryptedKey)

	for i := 0; i < 2; i++ {
		_, err := svc.Create(context.Background(), "org-1", models.CreateProviderCredentialInput{
			Provider: "OPENAI",
			APIKey:   "plain-key",
		})
		if err != nil {
			t.Fatalf("create credential %d failed: %v", i+1, err)
		}
	}

	if seeder.listCalls != 2 {
		t.Fatalf("expected seeder list called twice, got %d", seeder.listCalls)
	}
	if len(seeder.createCalls) != 7 {
		t.Fatalf("expected 7 seeded models, got %d", len(seeder.createCalls))
	}
	if seeder.createCalls[0].LevelGroup != models.ModelLevelFast || seeder.createCalls[2].LevelGroup != models.ModelLevelBalanced {
		t.Fatalf("unexpected default seed levels: %+v", seeder.createCalls)
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

func expectProviderCredentialByIDAndOrg(mock sqlmock.Sqlmock, id, orgID, provider, encryptedKey string) {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "org_id", "provider", "label", "encrypted_key", "base_url", "status", "priority", "metadata", "created_at", "updated_at",
	}).AddRow(id, orgID, provider, "default", encryptedKey, "", models.ProviderCredentialStatusActive, 0, []byte("{}"), now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "provider_credentials" WHERE id = $1 AND org_id = $2 ORDER BY "provider_credentials"."id" LIMIT $3`)).
		WithArgs(id, orgID, 1).
		WillReturnRows(rows)
}

func expectCreateCredential(mock sqlmock.Sqlmock, id, provider, encryptedKey string) {
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "provider_credentials"`)).
		WithArgs("org-1", provider, "default", sqlmock.AnyArg(), "", models.ProviderCredentialStatusActive, 0, nil, sqlmock.AnyArg(), sqlmock.AnyArg(), []byte("{}")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	mock.ExpectCommit()
}

func TestCredentialPoolService_SelectCredentialModelCooldown(t *testing.T) {
	svc, mock, encryptedKey, cleanup := newCredentialPoolServiceForTest(t, "plain-key")
	defer cleanup()

	creds := []models.ProviderCredential{
		{ID: "cred-1", OrgID: "org-1", Provider: "openai", Label: "default", EncryptedKey: encryptedKey, Status: models.ProviderCredentialStatusActive, Priority: 1},
		{ID: "cred-2", OrgID: "org-1", Provider: "openai", Label: "default", EncryptedKey: encryptedKey, Status: models.ProviderCredentialStatusActive, Priority: 2},
	}

	// 1. Initially, SelectCredential for "gpt-4o" should return "cred-1" (since priority 1 < 2)
	expectListActiveProviderCredentials(mock, "org-1", "openai", creds)
	cred, err := svc.SelectCredential(context.Background(), "org-1", "openai", "gpt-4o", StrategyFillFirst, nil)
	if err != nil {
		t.Fatalf("SelectCredential failed: %v", err)
	}
	if cred.ID != "cred-1" {
		t.Fatalf("expected cred-1, got %s", cred.ID)
	}

	// 2. Put "cred-1" on cooldown for model "gpt-4o"
	svc.SetCooldown(context.Background(), "cred-1", "gpt-4o", time.Now().Add(5*time.Minute))

	// 3. SelectCredential for "gpt-4o" should now skip "cred-1" and return "cred-2"
	expectListActiveProviderCredentials(mock, "org-1", "openai", creds)
	cred, err = svc.SelectCredential(context.Background(), "org-1", "openai", "gpt-4o", StrategyFillFirst, nil)
	if err != nil {
		t.Fatalf("SelectCredential failed: %v", err)
	}
	if cred.ID != "cred-2" {
		t.Fatalf("expected cred-2, got %s", cred.ID)
	}

	// 4. SelectCredential for another model, say "gpt-4o-mini", should still return "cred-1"
	// since the cooldown is model-specific!
	expectListActiveProviderCredentials(mock, "org-1", "openai", creds)
	cred, err = svc.SelectCredential(context.Background(), "org-1", "openai", "gpt-4o-mini", StrategyFillFirst, nil)
	if err != nil {
		t.Fatalf("SelectCredential failed: %v", err)
	}
	if cred.ID != "cred-1" {
		t.Fatalf("expected cred-1 for gpt-4o-mini, got %s", cred.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func expectListActiveProviderCredentials(mock sqlmock.Sqlmock, orgID, provider string, creds []models.ProviderCredential) {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "org_id", "provider", "label", "encrypted_key", "base_url", "status", "priority", "cooldown_until", "metadata", "created_at", "updated_at",
	})
	for _, c := range creds {
		rows.AddRow(c.ID, c.OrgID, c.Provider, c.Label, c.EncryptedKey, c.BaseURL, c.Status, c.Priority, c.CooldownUntil, c.Metadata, now, now)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "provider_credentials" WHERE (org_id = $1 AND provider = $2 AND status = $3) AND (cooldown_until IS NULL OR cooldown_until < NOW()) ORDER BY priority ASC, created_at ASC`)).
		WithArgs(orgID, provider, models.ProviderCredentialStatusActive).
		WillReturnRows(rows)
}
