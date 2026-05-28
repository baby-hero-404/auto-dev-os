package models

import "time"

type TokenUsage struct {
	ID           string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ProjectID    *string   `json:"project_id,omitempty" gorm:"type:uuid"`
	AgentID      *string   `json:"agent_id,omitempty" gorm:"type:uuid"`
	TaskID       *string   `json:"task_id,omitempty" gorm:"type:uuid"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Tier         string    `json:"tier"`
	PromptTokens int       `json:"prompt_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	LatencyMS    int64     `json:"latency_ms"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
}

func (TokenUsage) TableName() string {
	return "token_usage"
}

type TokenUsageSummary struct {
	ProjectID    *string `json:"project_id,omitempty"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Tier         string  `json:"tier"`
	Requests     int64   `json:"requests"`
	PromptTokens int64   `json:"prompt_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
}

type AgentSkill struct {
	AgentID   string    `json:"agent_id" gorm:"type:uuid;primaryKey"`
	SkillID   string    `json:"skill_id" gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `json:"created_at"`
}
