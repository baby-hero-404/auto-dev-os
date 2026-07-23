package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/attest"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newAttestationServiceTest(t *testing.T) (*AttestationService, sqlmock.Sqlmock, func()) {
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
	cipher, err := NewSecretCipher("test-key-material-for-attestation-tests")
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	repo := repository.NewAttestationRepo(gormDB)
	keyRepo := repository.NewAttestationKeyRepo(gormDB)
	svc := NewAttestationService(repo, keyRepo, cipher)
	cleanup := func() { _ = db.Close() }
	return svc, mock, cleanup
}

// TestAttestationService_SignThenVerify_RoundTrip exercises the full chain:
// sign a commit with a known (pre-seeded) active key -> verify it succeeds
// -> tamper the stored envelope's payload -> verify now fails
// (REQ-002/REQ-004).
func TestAttestationService_SignThenVerify_RoundTrip(t *testing.T) {
	svc, mock, cleanup := newAttestationServiceTest(t)
	defer cleanup()

	keyID, pub, priv, err := attest.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	cipher, _ := NewSecretCipher("test-key-material-for-attestation-tests")
	encPriv, err := cipher.Encrypt(base64.StdEncoding.EncodeToString(priv))
	if err != nil {
		t.Fatalf("failed to encrypt priv key: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	// SignCommit: EnsureActiveKey finds the pre-seeded active key, then
	// persists the signed attestation.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestation_keys" WHERE status = $1`)).
		WithArgs("active", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"key_id", "public_key", "private_key_encrypted", "status"}).
			AddRow(keyID, pubB64, encPriv, "active"))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "attestations"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("att-1"))
	mock.ExpectCommit()

	att, err := svc.SignCommit(context.Background(), SignInput{
		RepoName:   "org/repo",
		CommitHash: "deadbeef1234",
		TaskID:     "task-1",
		JobID:      "job-1",
		CodedBy:    attest.Actor{Provider: "anthropic", Model: "claude-sonnet-5"},
		PromptHash: "sha256:abc",
		Policy:     attest.PolicySnapshot{Autonomy: "supervised", FixCyclesUsed: 1},
	})
	if err != nil {
		t.Fatalf("SignCommit failed: %v", err)
	}
	if att.KeyID != keyID {
		t.Fatalf("expected attestation to record key_id %q, got %q", keyID, att.KeyID)
	}

	// VerifyByCommitHash: look up the attestation, then look up the exact
	// key it recorded (not "whatever's active now").
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestations" WHERE commit_hash = $1`)).
		WithArgs("deadbeef1234", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_id", "commit_hash", "envelope"}).
			AddRow("att-1", att.KeyID, "deadbeef1234", att.Envelope))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestation_keys" WHERE key_id = $1`)).
		WithArgs(att.KeyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"key_id", "public_key", "status"}).
			AddRow(keyID, pubB64, "active"))

	result, err := svc.VerifyByCommitHash(context.Background(), "deadbeef1234")
	if err != nil {
		t.Fatalf("VerifyByCommitHash failed: %v", err)
	}
	if !result.Verified {
		t.Fatal("expected verification to succeed on untampered envelope")
	}

	// Tamper: flip a byte in the decoded payload, re-encode, verify now fails.
	var tamperedEnv attest.Envelope
	if err := json.Unmarshal(att.Envelope, &tamperedEnv); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	rawPayload, err := base64.StdEncoding.DecodeString(tamperedEnv.Payload)
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	rawPayload[len(rawPayload)-2] ^= 0xFF
	tamperedEnv.Payload = base64.StdEncoding.EncodeToString(rawPayload)
	tamperedJSON, err := json.Marshal(tamperedEnv)
	if err != nil {
		t.Fatalf("failed to marshal tampered envelope: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestations" WHERE commit_hash = $1`)).
		WithArgs("deadbeef1234", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_id", "commit_hash", "envelope"}).
			AddRow("att-1", att.KeyID, "deadbeef1234", tamperedJSON))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestation_keys" WHERE key_id = $1`)).
		WithArgs(att.KeyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"key_id", "public_key", "status"}).
			AddRow(keyID, pubB64, "active"))

	tamperedResult, err := svc.VerifyByCommitHash(context.Background(), "deadbeef1234")
	if err != nil {
		t.Fatalf("VerifyByCommitHash (tampered) returned unexpected error: %v", err)
	}
	if tamperedResult.Verified {
		t.Fatal("expected verification to fail on tampered envelope payload")
	}
	if tamperedResult.Attestation.CreatedAt.After(time.Now()) {
		t.Fatalf("unexpected created_at in future: %v", tamperedResult.Attestation.CreatedAt)
	}
}

// TestAttestationService_RotateKey_OldRecordsStillVerify confirms that after
// rotating the signing key, an attestation signed by the old (now retired)
// key still verifies, because verification always looks up the key_id
// recorded on the record itself (REQ-004/REQ-006).
func TestAttestationService_RotateKey_OldRecordsStillVerify(t *testing.T) {
	svc, mock, cleanup := newAttestationServiceTest(t)
	defer cleanup()

	oldKeyID, oldPub, oldPriv, err := attest.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate old keypair: %v", err)
	}
	oldPubB64 := base64.StdEncoding.EncodeToString(oldPub)

	// Build and sign a statement with the old key, as if it were signed
	// before rotation happened.
	predicate := attest.Predicate{TaskID: "task-1", JobID: "job-1", Timestamp: time.Now().UTC()}
	statement := attest.BuildStatement("org/repo", "oldcommit", predicate)
	signedEnv, err := attest.Sign(statement, oldKeyID, oldPriv)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	envelopeJSON, _ := json.Marshal(signedEnv)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestations" WHERE commit_hash = $1`)).
		WithArgs("oldcommit", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "key_id", "commit_hash", "envelope"}).
			AddRow("att-old", oldKeyID, "oldcommit", envelopeJSON))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "attestation_keys" WHERE key_id = $1`)).
		WithArgs(oldKeyID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"key_id", "public_key", "status"}).
			AddRow(oldKeyID, oldPubB64, "retired"))

	result, err := svc.VerifyByCommitHash(context.Background(), "oldcommit")
	if err != nil {
		t.Fatalf("VerifyByCommitHash failed: %v", err)
	}
	if !result.Verified {
		t.Fatal("expected a retired key to still verify records it originally signed")
	}
}
