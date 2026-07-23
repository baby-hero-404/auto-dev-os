package models

import (
	"encoding/json"
	"time"
)

// AttestationKeyStatus mirrors attest.KeyStatus for GORM/API use.
type AttestationKeyStatus string

const (
	AttestationKeyActive  AttestationKeyStatus = "active"
	AttestationKeyRetired AttestationKeyStatus = "retired"
)

// AttestationKey is a per-deployment Ed25519 signing key (REQ-006). The
// private key is stored encrypted at rest via the existing SecretCipher.
type AttestationKey struct {
	KeyID               string               `json:"key_id" gorm:"column:key_id;primaryKey"`
	PublicKey           string               `json:"public_key" gorm:"column:public_key;not null"`
	PrivateKeyEncrypted string               `json:"-" gorm:"column:private_key_encrypted;not null"`
	Status              AttestationKeyStatus `json:"status" gorm:"column:status;not null;default:active"`
	CreatedAt           time.Time            `json:"created_at" gorm:"column:created_at"`
}

func (AttestationKey) TableName() string { return "attestation_keys" }

// Attestation is one signed DSSE envelope covering a single commit
// (REQ-001/REQ-002).
type Attestation struct {
	ID             string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TaskID         string          `json:"task_id" gorm:"column:task_id;type:uuid;not null"`
	JobID          string          `json:"job_id" gorm:"column:job_id;type:uuid"`
	CommitHash     string          `json:"commit_hash" gorm:"column:commit_hash;not null"`
	KeyID          string          `json:"key_id" gorm:"column:key_id;not null"`
	CodedBy        json.RawMessage `json:"coded_by" gorm:"column:coded_by;type:jsonb"`
	ReviewedBy     json.RawMessage `json:"reviewed_by" gorm:"column:reviewed_by;type:jsonb"`
	PromptHash     string          `json:"prompt_hash" gorm:"column:prompt_hash"`
	PolicySnapshot json.RawMessage `json:"policy_snapshot" gorm:"column:policy_snapshot;type:jsonb"`
	Envelope       json.RawMessage `json:"envelope" gorm:"column:envelope;type:jsonb;not null"`
	CreatedAt      time.Time       `json:"created_at" gorm:"column:created_at"`
}

func (Attestation) TableName() string { return "attestations" }

// CreateAttestationInput is the input to AttestationRepo.Create.
type CreateAttestationInput struct {
	TaskID         string
	JobID          string
	CommitHash     string
	KeyID          string
	CodedBy        json.RawMessage
	ReviewedBy     json.RawMessage
	PromptHash     string
	PolicySnapshot json.RawMessage
	Envelope       json.RawMessage
}
