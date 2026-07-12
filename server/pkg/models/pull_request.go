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
	Title               string   `json:"title"`
	Body                string   `json:"body"`
	PRURL               string   `json:"pr_url"`
	ChangedFiles        []string `json:"changed_files"`
	RiskLevel           string   `json:"risk_level"`
	RiskReason          string   `json:"risk_reason"`
	RiskDomains         []string `json:"risk_domains,omitempty"` // Areas of risk impact (e.g., "security", "performance", "api_contract")
	Status              string   `json:"status"`
	ReviewLimitExceeded bool     `json:"review_limit_exceeded"`
	SelfReviewFallback  bool     `json:"self_review_fallback"` // true if Harness Independence had no alternative model and reused the coder's model for review
}

// PRRejectInput is the payload for rejecting a PR.
type PRRejectInput struct {
	Feedback string `json:"feedback"`
}
