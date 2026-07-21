package models

import (
	"encoding/json"
	"time"
)

const (
	ProviderCredentialStatusActive      = "active"
	ProviderCredentialStatusRateLimited = "rate_limited"
	ProviderCredentialStatusDisabled    = "disabled"
)

type ProviderCredential struct {
	ID            string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID         string          `json:"org_id" gorm:"type:uuid;not null"`
	Provider      string          `json:"provider" gorm:"not null"`
	Label         string          `json:"label" gorm:"default:'default';not null"`
	EncryptedKey  string          `json:"-" gorm:"column:encrypted_key;not null"`
	BaseURL       string          `json:"base_url,omitempty"`
	Status        string          `json:"status" gorm:"default:'active';not null"`
	Priority      int             `json:"priority" gorm:"default:0"`
	CooldownUntil *time.Time      `json:"cooldown_until,omitempty"`
	Metadata      json.RawMessage `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CreateProviderCredentialInput struct {
	Provider string          `json:"provider"`
	Label    string          `json:"label"`
	APIKey   string          `json:"api_key"`
	BaseURL  string          `json:"base_url,omitempty"`
	Priority int             `json:"priority"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type TestProviderCredentialInput struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url,omitempty"`
}

type UpdateProviderCredentialInput struct {
	Label    *string          `json:"label,omitempty"`
	APIKey   *string          `json:"api_key,omitempty"`
	BaseURL  *string          `json:"base_url,omitempty"`
	Status   *string          `json:"status,omitempty"`
	Priority *int             `json:"priority,omitempty"`
	Metadata *json.RawMessage `json:"metadata,omitempty"`
}

type ProviderCredentialResponse struct {
	ID             string               `json:"id"`
	Provider       string               `json:"provider"`
	Label          string               `json:"label"`
	BaseURL        string               `json:"base_url,omitempty"`
	Status         string               `json:"status"`
	Priority       int                  `json:"priority"`
	Configured     bool                 `json:"configured"`
	KeySuffix      string               `json:"key_suffix,omitempty"`
	CooldownUntil  *time.Time           `json:"cooldown_until,omitempty"`
	ModelCooldowns map[string]time.Time `json:"model_cooldowns,omitempty"`
	Metadata       json.RawMessage      `json:"metadata,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

func (c ProviderCredential) ToResponse(keySuffix string) ProviderCredentialResponse {
	return ProviderCredentialResponse{
		ID:            c.ID,
		Provider:      c.Provider,
		Label:         c.Label,
		BaseURL:       c.BaseURL,
		Status:        c.Status,
		Priority:      c.Priority,
		Configured:    c.EncryptedKey != "",
		KeySuffix:     keySuffix,
		CooldownUntil: c.CooldownUntil,
		Metadata:      c.Metadata,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}
