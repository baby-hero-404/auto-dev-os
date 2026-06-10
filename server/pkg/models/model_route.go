package models

import (
	"encoding/json"
	"time"
)

const (
	ModelRouteTypeTier  = "tier"
	ModelRouteTypeCombo = "combo"
)

type ModelRoute struct {
	ID        string          `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID     string          `json:"org_id" gorm:"type:uuid;not null"`
	Name      string          `json:"name" gorm:"not null"`
	RouteType string          `json:"route_type" gorm:"not null"`
	Config    json.RawMessage `json:"config" gorm:"type:jsonb;not null"`
	IsDefault bool            `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type ComboEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Priority int    `json:"priority"`
	Tier     string `json:"tier,omitempty"`
}

type CreateModelRouteInput struct {
	Name      string          `json:"name"`
	RouteType string          `json:"route_type"`
	Config    json.RawMessage `json:"config"`
	IsDefault bool            `json:"is_default"`
}

type UpdateModelRouteInput struct {
	Name      *string          `json:"name,omitempty"`
	RouteType *string          `json:"route_type,omitempty"`
	Config    *json.RawMessage `json:"config,omitempty"`
	IsDefault *bool            `json:"is_default,omitempty"`
}
