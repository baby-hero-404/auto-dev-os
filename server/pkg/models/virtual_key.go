package models

import "time"

const (
	VirtualKeyStatusActive    = "active"
	VirtualKeyStatusExhausted = "exhausted"
	VirtualKeyStatusRevoked   = "revoked"
)

type VirtualKey struct {
	ID             string     `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID          string     `json:"org_id" gorm:"type:uuid;not null"`
	ProjectID      *string    `json:"project_id,omitempty" gorm:"type:uuid"`
	AgentID        *string    `json:"agent_id,omitempty" gorm:"type:uuid"`
	KeyHash        string     `json:"-" gorm:"not null;unique"`
	KeyPrefix      string     `json:"key_prefix" gorm:"not null"`
	Name           string     `json:"name" gorm:"not null"`
	BudgetLimitUSD *float64   `json:"budget_limit_usd,omitempty"`
	BudgetUsedUSD  float64    `json:"budget_used_usd" gorm:"default:0"`
	RPMLimit       *int       `json:"rpm_limit,omitempty"`
	TPMLimit       *int       `json:"tpm_limit,omitempty"`
	Status         string     `json:"status" gorm:"default:'active';not null"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateVirtualKeyInput struct {
	Name           string     `json:"name"`
	ProjectID      *string    `json:"project_id,omitempty"`
	AgentID        *string    `json:"agent_id,omitempty"`
	BudgetLimitUSD *float64   `json:"budget_limit_usd,omitempty"`
	RPMLimit       *int       `json:"rpm_limit,omitempty"`
	TPMLimit       *int       `json:"tpm_limit,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type UpdateVirtualKeyInput struct {
	Name           *string    `json:"name,omitempty"`
	BudgetLimitUSD *float64   `json:"budget_limit_usd,omitempty"`
	RPMLimit       *int       `json:"rpm_limit,omitempty"`
	TPMLimit       *int       `json:"tpm_limit,omitempty"`
	Status         *string    `json:"status,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type VirtualKeyResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"key_prefix"`
	ProjectID      *string    `json:"project_id,omitempty"`
	AgentID        *string    `json:"agent_id,omitempty"`
	BudgetLimitUSD *float64   `json:"budget_limit_usd,omitempty"`
	BudgetUsedUSD  float64    `json:"budget_used_usd"`
	RPMLimit       *int       `json:"rpm_limit,omitempty"`
	TPMLimit       *int       `json:"tpm_limit,omitempty"`
	Status         string     `json:"status"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type CreatedVirtualKeyResponse struct {
	VirtualKeyResponse
	Key string `json:"key"`
}

func (k VirtualKey) ToResponse() VirtualKeyResponse {
	return VirtualKeyResponse{
		ID:             k.ID,
		Name:           k.Name,
		KeyPrefix:      k.KeyPrefix,
		ProjectID:      k.ProjectID,
		AgentID:        k.AgentID,
		BudgetLimitUSD: k.BudgetLimitUSD,
		BudgetUsedUSD:  k.BudgetUsedUSD,
		RPMLimit:       k.RPMLimit,
		TPMLimit:       k.TPMLimit,
		Status:         k.Status,
		ExpiresAt:      k.ExpiresAt,
		CreatedAt:      k.CreatedAt,
	}
}
