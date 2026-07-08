package policy

import (
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

var HighRiskPatterns = map[string][]string{
	"auth":           {"auth/", "login", "jwt", "oauth", "session", "token"},
	"payment":        {"payment", "billing", "stripe", "invoice", "pricing"},
	"data_migration": {"migration", "migrations/", "schema", "backfill", "etl"},
	"infra":          {"Dockerfile", "docker-compose", ".github/workflows", "deploy", "terraform", "k8s"},
	"security":       {"secret", "cors", "csp", "encryption", "vulnerability"},
	"rbac":           {"permission", "rbac", "role", "policy", "access_control"},
	"public_api":     {"api/", "openapi", "proto", "graphql"},
}

// HasHighRiskDomains checks affected files and risk_domains from analysis.
func HasHighRiskDomains(affectedFiles []string, riskDomains []string) bool {
	// 1. Check risk domains
	for _, rd := range riskDomains {
		rdLower := strings.ToLower(strings.TrimSpace(rd))
		if _, exists := HighRiskPatterns[rdLower]; exists {
			return true
		}
	}

	// 2. Check affected files
	for _, file := range affectedFiles {
		fileLower := strings.ToLower(file)
		for _, patterns := range HighRiskPatterns {
			for _, pattern := range patterns {
				if strings.Contains(fileLower, strings.ToLower(pattern)) {
					return true
				}
			}
		}
	}

	return false
}

// ShouldAutoApproveSpec determines whether a spec can be auto-approved.
// Returns (specStatus, taskStatus).
func ShouldAutoApproveSpec(
	complexity string,
	affectedFiles []string,
	riskDomains []string,
	agentAutonomy string,
	projectAutonomy string,
	projectReviewPolicy string,
	hasClarifications bool,
) (specStatus string, taskStatus string) {
	if hasClarifications {
		return models.TaskSpecStatusClarificationRequired, models.TaskStatusSpecReview
	}

	if projectReviewPolicy == "always_review" {
		return models.TaskSpecStatusPendingReview, models.TaskStatusSpecReview
	}

	// Risk check - if high risk, always require review regardless of autonomy or auto_merge policy
	if HasHighRiskDomains(affectedFiles, riskDomains) {
		return models.TaskSpecStatusPendingReview, models.TaskStatusSpecReview
	}

	if projectReviewPolicy == "auto_merge" {
		return models.TaskSpecStatusAutoApproved, models.TaskStatusCoding
	}

	// Default/complexity_based policy
	autonomy := agentAutonomy
	if autonomy == "" {
		autonomy = projectAutonomy
	}
	if autonomy == "" {
		autonomy = models.AgentAutonomySupervised
	}

	switch autonomy {
	case models.AgentAutonomyAutonomous:
		return models.TaskSpecStatusAutoApproved, models.TaskStatusCoding
	case models.AgentAutonomyApprovalRequired:
		return models.TaskSpecStatusPendingReview, models.TaskStatusSpecReview
	default: // supervised / complexity-based default
		if complexity == models.TaskComplexityEasy {
			return models.TaskSpecStatusAutoApproved, models.TaskStatusCoding
		}
		return models.TaskSpecStatusPendingReview, models.TaskStatusSpecReview
	}
}
