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

// MaxClarificationRounds caps how many times the clarification-required gate
// can pause a task on the same unanswered questions before it gives up and
// lets the pipeline proceed anyway — a task whose author won't (or can't)
// answer clarifications must not deadlock in spec_review forever.
const MaxClarificationRounds = 2

// IsDefinitionOfReadyBypassed reports whether the definition-of-ready gate's
// clarification-required block should be skipped for this task: either the
// task is flagged as a hotfix (governance intentionally trusts hotfixes to
// skip friction), or the task has already cycled through
// MaxClarificationRounds of unanswered/still-insufficient clarifications and
// blocking further would just deadlock the pipeline.
func IsDefinitionOfReadyBypassed(labels []string, priorClarificationRounds int) bool {
	if priorClarificationRounds >= MaxClarificationRounds {
		return true
	}
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), "hotfix") {
			return true
		}
	}
	return false
}

// ShouldAutoApproveSpec determines whether a spec can be auto-approved.
// Returns (specStatus, taskStatus). When hasClarifications is true but the
// caller has determined via IsDefinitionOfReadyBypassed that the gate should
// be bypassed (hotfix label, or clarification round limit reached), the
// caller should pass hasClarifications=false and instead surface the bypass
// to the user via TaskSpecStatusReadyWithWarnings + a logged warning.
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
