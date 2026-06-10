package models

// PR risk levels.
const (
	PRRiskLow      = "low"
	PRRiskMedium   = "medium"
	PRRiskHigh     = "high"
	PRRiskCritical = "critical"
)

// PR review statuses.
const (
	PRStatusOpen     = "open"
	PRStatusApproved = "approved"
	PRStatusRejected = "rejected"
	PRStatusMerged   = "merged"
)

// PRSummary represents AI-generated PR information attached to a task.
type PRSummary struct {
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	PRURL        string   `json:"pr_url"`
	ChangedFiles []string `json:"changed_files"`
	RiskLevel    string   `json:"risk_level"`
	RiskReason   string   `json:"risk_reason"`
	Status       string   `json:"status"`
}

// PRRejectInput is the payload for rejecting a PR.
type PRRejectInput struct {
	Feedback string `json:"feedback"`
}
