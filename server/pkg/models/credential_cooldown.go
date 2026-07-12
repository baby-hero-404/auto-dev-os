package models

import "time"

// CredentialCooldown persists per-(credential, model) cooldown state so it survives
// process restarts and is visible across horizontally-scaled API replicas, instead of
// living only in an in-process map (see server/internal/service/credential_pool.go).
type CredentialCooldown struct {
	CredentialID  string    `gorm:"column:credential_id;primaryKey" json:"credential_id"`
	Model         string    `gorm:"column:model;primaryKey" json:"model"`
	CooldownUntil time.Time `gorm:"column:cooldown_until" json:"cooldown_until"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CredentialCooldown) TableName() string { return "credential_cooldowns" }
