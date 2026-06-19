# Backlog: Virtual Key Architecture

This document preserves the designed architecture, database schemas, and codebase patterns for the **Virtual Key Management** feature. This feature has been deferred and removed from the active development codebase to reduce surface area, but can be re-implemented in the future using this specification.

---

## 1. Feature Overview
Virtual Keys allow the generation of scoped, hashed keys (e.g. `sk-aco-XXXX`) for agents, projects, or users. This enables:
- Scoped access to the Unified AI Gateway.
- Budget cap enforcement (hard and soft limits) per agent or project.
- Rate limiting (TPM/RPM) isolated to individual execution domains.
- Full auditability of LLM consumption without exposing raw provider API keys.

---

## 2. Database Schema (PostgreSQL)

```sql
CREATE TYPE virtual_key_status AS ENUM ('active', 'exhausted', 'revoked');

CREATE TABLE virtual_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    budget_limit_usd DECIMAL(10, 4),
    budget_used_usd DECIMAL(10, 4) NOT NULL DEFAULT 0.0000,
    rpm_limit INT,
    tpm_limit INT,
    status virtual_key_status NOT NULL DEFAULT 'active',
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_virtual_keys_hash ON virtual_keys(key_hash);
CREATE INDEX idx_virtual_keys_org ON virtual_keys(org_id);
```

---

## 3. Go Models (`server/pkg/models/virtual_key.go`)

```go
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
	Name           string     `json:"name" gorm:"not null"`
	KeyHash        string     `json:"-" gorm:"not null;uniqueIndex"`
	KeyPrefix      string     `json:"key_prefix" gorm:"not null"`
	BudgetLimitUSD *float64   `json:"budget_limit_usd,omitempty"`
	BudgetUsedUSD  float64    `json:"budget_used_usd" gorm:"default:0.0"`
	RPMLimit       *int       `json:"rpm_limit,omitempty"`
	TPMLimit       *int       `json:"tpm_limit,omitempty"`
	Status         string     `json:"status" gorm:"default:'active'"`
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
```

---

## 4. Frontend Types (`web/src/lib/types.ts`)

```typescript
export type VirtualKey = {
  id: string;
  name: string;
  key_prefix: string;
  project_id?: string;
  agent_id?: string;
  budget_limit_usd?: number;
  budget_used_usd: number;
  rpm_limit?: number;
  tpm_limit?: number;
  status: "active" | "exhausted" | "revoked";
  expires_at?: string;
  created_at: string;
};

export type CreatedVirtualKey = VirtualKey & {
  key: string;
};

export type CreateVirtualKeyInput = {
  name: string;
  project_id?: string;
  agent_id?: string;
  budget_limit_usd?: number;
  rpm_limit?: number;
  tpm_limit?: number;
  expires_at?: string;
};

export type UpdateVirtualKeyInput = {
  name?: string;
  budget_limit_usd?: number;
  rpm_limit?: number;
  tpm_limit?: number;
  status?: VirtualKey["status"];
  expires_at?: string;
};
```

---

## 5. API Client (`web/src/lib/api/gateway.ts`)

```typescript
export const virtualKeys = {
  list(orgID: string, token: string) {
    return request<VirtualKey[]>(`/organizations/${orgID}/virtual-keys`, { token });
  },
  create(orgID: string, token: string, input: CreateVirtualKeyInput) {
    return request<CreatedVirtualKey>(`/organizations/${orgID}/virtual-keys`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  get(orgID: string, virtualKeyID: string, token: string) {
    return request<VirtualKey>(`/organizations/${orgID}/virtual-keys/${virtualKeyID}`, { token });
  },
  update(orgID: string, virtualKeyID: string, token: string, input: UpdateVirtualKeyInput) {
    return request<VirtualKey>(`/organizations/${orgID}/virtual-keys/${virtualKeyID}`, {
      method: "PUT",
      token,
      body: JSON.stringify(input),
    });
  },
  revoke(orgID: string, virtualKeyID: string, token: string) {
    return request<void>(`/organizations/${orgID}/virtual-keys/${virtualKeyID}`, {
      method: "DELETE",
      token,
    });
  },
};
```
