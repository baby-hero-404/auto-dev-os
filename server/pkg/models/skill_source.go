package models

import "time"

type SkillSource struct {
	ID           string     `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	URL          string     `json:"url" gorm:"uniqueIndex;not null"`
	Status       string     `json:"status" gorm:"default:'pending';not null"`
	Error        string     `json:"error" gorm:"default:''"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateSkillSourceInput struct {
	URL string `json:"url"`
}

type UpdateSkillSourceInput struct {
	Status       *string    `json:"status,omitempty"`
	Error        *string    `json:"error,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

type FileItem struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type FileContent struct {
	Content string `json:"content"`
	Path    string `json:"path"`
}
