package models

import "time"

type Secret struct {
	ID        string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID string    `json:"project_id" gorm:"type:uuid;not null"`
	Name      string    `json:"name" gorm:"not null"`
	Value     string    `json:"-" gorm:"column:encrypted_value;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateSecretInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
