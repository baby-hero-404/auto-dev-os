package models

import "time"

// Project groups repositories, agents, and rules under an organization.
type Project struct {
	ID                 string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID              string    `json:"org_id" gorm:"type:uuid;not null"`
	Name               string    `json:"name" gorm:"not null"`
	Description        string    `json:"description" gorm:"default:''"`
	DefaultModelLevel  string    `json:"default_model_level" gorm:"column:default_model_level;default:'balanced';not null"`
	DefaultAutonomy    string    `json:"default_autonomy" gorm:"column:default_autonomy;default:'supervised';not null"`
	AutoReviewPolicy   string    `json:"auto_review_policy" gorm:"column:auto_review_policy;default:'complexity_based';not null"`
	MaxRetries         int       `json:"max_retries" gorm:"column:max_retries;default:3;not null"`
	MaxReviewFixCycles int       `json:"max_review_fix_cycles" gorm:"column:max_review_fix_cycles;default:3;not null"`
	DefaultBranch      string    `json:"default_branch" gorm:"column:default_branch;default:'main';not null"`
	RepositoriesCount  int       `json:"repositories_count,omitempty" gorm:"->"`
	AgentsCount        int       `json:"agents_count,omitempty" gorm:"->"`
	TasksDoneCount     int       `json:"tasks_done_count,omitempty" gorm:"->"`
	TasksTotalCount    int       `json:"tasks_total_count,omitempty" gorm:"->"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateProjectInput is the payload to create a project.
type CreateProjectInput struct {
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	DefaultModelLevel  *string `json:"default_model_level,omitempty"`
	DefaultAutonomy    *string `json:"default_autonomy,omitempty"`
	AutoReviewPolicy   *string `json:"auto_review_policy,omitempty"`
	MaxRetries         *int    `json:"max_retries,omitempty"`
	MaxReviewFixCycles *int    `json:"max_review_fix_cycles,omitempty"`
	DefaultBranch      *string `json:"default_branch,omitempty"`
}

// UpdateProjectInput is the payload to partially update a project.
type UpdateProjectInput struct {
	Name               *string `json:"name,omitempty"`
	Description        *string `json:"description,omitempty"`
	DefaultModelLevel  *string `json:"default_model_level,omitempty"`
	DefaultAutonomy    *string `json:"default_autonomy,omitempty"`
	AutoReviewPolicy   *string `json:"auto_review_policy,omitempty"`
	MaxRetries         *int    `json:"max_retries,omitempty"`
	MaxReviewFixCycles *int    `json:"max_review_fix_cycles,omitempty"`
	DefaultBranch      *string `json:"default_branch,omitempty"`
}
