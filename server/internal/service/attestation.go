package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/attest"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

// AttestationService signs per-commit attestations with the deployment's
// active Ed25519 key and verifies them against whichever key (active or
// retired) actually signed them (REQ-002/REQ-004/REQ-006).
type AttestationService struct {
	repo    *repository.AttestationRepo
	keyRepo *repository.AttestationKeyRepo
	cipher  *SecretCipher
}

func NewAttestationService(repo *repository.AttestationRepo, keyRepo *repository.AttestationKeyRepo, cipher *SecretCipher) *AttestationService {
	return &AttestationService{repo: repo, keyRepo: keyRepo, cipher: cipher}
}

// EnsureActiveKey returns the current active signing key, generating and
// persisting a new Ed25519 keypair on first use if none exists yet.
func (s *AttestationService) EnsureActiveKey(ctx context.Context) (*models.AttestationKey, error) {
	key, err := s.keyRepo.GetActive(ctx)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load active attestation key: %w", err)
	}

	keyID, pub, priv, err := attest.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	encPriv, err := s.cipher.Encrypt(base64.StdEncoding.EncodeToString(priv))
	if err != nil {
		return nil, fmt.Errorf("encrypt signing key: %w", err)
	}
	newKey := &models.AttestationKey{
		KeyID:               keyID,
		PublicKey:           base64.StdEncoding.EncodeToString(pub),
		PrivateKeyEncrypted: encPriv,
		Status:              models.AttestationKeyActive,
	}
	if err := s.keyRepo.Create(ctx, newKey); err != nil {
		return nil, err
	}
	return newKey, nil
}

// RotateKey retires every currently-active key and generates a new active
// one. Retired keys remain in the keyset so records they signed still
// verify (REQ-006).
func (s *AttestationService) RotateKey(ctx context.Context) (*models.AttestationKey, error) {
	if err := s.keyRepo.RetireActiveKeys(ctx); err != nil {
		return nil, err
	}
	keyID, pub, priv, err := attest.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	encPriv, err := s.cipher.Encrypt(base64.StdEncoding.EncodeToString(priv))
	if err != nil {
		return nil, fmt.Errorf("encrypt signing key: %w", err)
	}
	newKey := &models.AttestationKey{
		KeyID:               keyID,
		PublicKey:           base64.StdEncoding.EncodeToString(pub),
		PrivateKeyEncrypted: encPriv,
		Status:              models.AttestationKeyActive,
	}
	if err := s.keyRepo.Create(ctx, newKey); err != nil {
		return nil, err
	}
	return newKey, nil
}

// SignInput carries everything needed to build and sign one commit's
// Statement.
type SignInput struct {
	RepoName   string
	CommitHash string
	TaskID     string
	JobID      string
	CodedBy    attest.Actor
	ReviewedBy *attest.Actor
	PromptHash string
	Policy     attest.PolicySnapshot
}

// SignCommit builds a Statement for the given input, signs it with the
// active key, and persists the resulting attestation (REQ-001).
func (s *AttestationService) SignCommit(ctx context.Context, in SignInput) (*models.Attestation, error) {
	key, err := s.EnsureActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	privB64, err := s.cipher.Decrypt(key.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt active signing key: %w", err)
	}
	priv, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return nil, fmt.Errorf("decode active signing key: %w", err)
	}

	predicate := attest.Predicate{
		CodedBy:    in.CodedBy,
		ReviewedBy: in.ReviewedBy,
		PromptHash: in.PromptHash,
		Policy:     in.Policy,
		TaskID:     in.TaskID,
		JobID:      in.JobID,
		Timestamp:  time.Now().UTC(),
	}
	statement := attest.BuildStatement(in.RepoName, in.CommitHash, predicate)

	env, err := attest.Sign(statement, key.KeyID, ed25519.PrivateKey(priv))
	if err != nil {
		return nil, fmt.Errorf("sign attestation: %w", err)
	}
	envelopeJSON, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	codedByJSON, _ := json.Marshal(in.CodedBy)
	var reviewedByJSON json.RawMessage
	if in.ReviewedBy != nil {
		reviewedByJSON, _ = json.Marshal(in.ReviewedBy)
	}
	policyJSON, _ := json.Marshal(in.Policy)

	return s.repo.Create(ctx, models.CreateAttestationInput{
		TaskID:         in.TaskID,
		JobID:          in.JobID,
		CommitHash:     in.CommitHash,
		KeyID:          key.KeyID,
		CodedBy:        codedByJSON,
		ReviewedBy:     reviewedByJSON,
		PromptHash:     in.PromptHash,
		PolicySnapshot: policyJSON,
		Envelope:       envelopeJSON,
	})
}

// VerifyResult is the outcome of verifying one commit's attestation.
type VerifyResult struct {
	Attestation *models.Attestation
	Envelope    *attest.Envelope
	Verified    bool
	KeyID       string
}

// VerifyByCommitHash looks up the attestation for commitHash and verifies
// its signature using the key_id recorded on the attestation itself —
// never the deployment's current active key — so rotated-out keys still
// verify their own historical records (REQ-004).
func (s *AttestationService) VerifyByCommitHash(ctx context.Context, commitHash string) (*VerifyResult, error) {
	a, err := s.repo.GetByCommitHash(ctx, commitHash)
	if err != nil {
		return nil, err
	}
	var env attest.Envelope
	if err := json.Unmarshal(a.Envelope, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	key, err := s.keyRepo.GetByKeyID(ctx, a.KeyID)
	if err != nil {
		return nil, fmt.Errorf("load signing key %q: %w", a.KeyID, err)
	}
	pub, err := base64.StdEncoding.DecodeString(key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode key %q public key: %w", key.KeyID, err)
	}
	verifyErr := attest.Verify(&env, a.KeyID, ed25519.PublicKey(pub))
	return &VerifyResult{
		Attestation: a,
		Envelope:    &env,
		Verified:    verifyErr == nil,
		KeyID:       a.KeyID,
	}, nil
}

// ListByTaskID returns every attestation recorded for a task's commits, for
// the Audit panel's coded_by -> reviewed_by -> attested chain (REQ-005).
func (s *AttestationService) ListByTaskID(ctx context.Context, taskID string) ([]models.Attestation, error) {
	return s.repo.ListByTaskID(ctx, taskID)
}

// JWKS returns the deployment's full keyset (active + retired) in
// JWKS-like form for offline verification (REQ-006).
func (s *AttestationService) JWKS(ctx context.Context) (*attest.JWKSet, error) {
	keys, err := s.keyRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := &attest.JWKSet{}
	for _, k := range keys {
		pub, err := base64.StdEncoding.DecodeString(k.PublicKey)
		if err != nil {
			continue
		}
		out.Keys = append(out.Keys, attest.JWK{
			Kty:    "OKP",
			Crv:    "Ed25519",
			X:      base64.RawURLEncoding.EncodeToString(pub),
			KeyID:  k.KeyID,
			Status: string(k.Status),
		})
	}
	return out, nil
}
