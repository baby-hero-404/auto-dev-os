package policy

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestHasHighRiskDomains(t *testing.T) {
	tests := []struct {
		name          string
		affectedFiles []string
		riskDomains   []string
		expected      bool
	}{
		{
			name:          "no risk",
			affectedFiles: []string{"server/internal/handler/health.go"},
			riskDomains:   nil,
			expected:      false,
		},
		{
			name:          "risk domain match",
			affectedFiles: []string{"server/internal/handler/health.go"},
			riskDomains:   []string{"auth"},
			expected:      true,
		},
		{
			name:          "risk file path match auth",
			affectedFiles: []string{"server/internal/auth/handler.go"},
			riskDomains:   nil,
			expected:      true,
		},
		{
			name:          "risk file path match login substring",
			affectedFiles: []string{"web/components/LoginForm.tsx"},
			riskDomains:   nil,
			expected:      true,
		},
		{
			name:          "risk file path match infra Dockerfile",
			affectedFiles: []string{"docker/Dockerfile"},
			riskDomains:   nil,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := HasHighRiskDomains(tt.affectedFiles, tt.riskDomains)
			if actual != tt.expected {
				t.Errorf("HasHighRiskDomains() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestShouldAutoApproveSpec(t *testing.T) {
	tests := []struct {
		name                string
		complexity          string
		affectedFiles       []string
		riskDomains         []string
		agentAutonomy       string
		projectAutonomy     string
		projectReviewPolicy string
		hasClarifications   bool
		wantSpecStatus      string
		wantTaskStatus      string
	}{
		{
			name:              "has clarifications",
			complexity:        models.TaskComplexityEasy,
			hasClarifications: true,
			wantSpecStatus:    models.TaskSpecStatusChangesRequested,
			wantTaskStatus:    models.TaskStatusSpecReview,
		},
		{
			name:                "always review policy",
			complexity:          models.TaskComplexityEasy,
			projectReviewPolicy: "always_review",
			wantSpecStatus:      models.TaskSpecStatusPendingReview,
			wantTaskStatus:      models.TaskStatusSpecReview,
		},
		{
			name:          "high risk domains overrides autonomy and policy",
			complexity:    models.TaskComplexityEasy,
			affectedFiles: []string{"server/internal/auth/handler.go"},
			agentAutonomy: models.AgentAutonomyAutonomous,
			wantSpecStatus: models.TaskSpecStatusPendingReview,
			wantTaskStatus: models.TaskStatusSpecReview,
		},
		{
			name:                "auto merge policy",
			complexity:          models.TaskComplexityMedium,
			projectReviewPolicy: "auto_merge",
			wantSpecStatus:      models.TaskSpecStatusAutoApproved,
			wantTaskStatus:      models.TaskStatusCoding,
		},
		{
			name:            "agent autonomy autonomous",
			complexity:      models.TaskComplexityMedium,
			agentAutonomy:   models.AgentAutonomyAutonomous,
			wantSpecStatus:  models.TaskSpecStatusAutoApproved,
			wantTaskStatus:  models.TaskStatusCoding,
		},
		{
			name:            "agent autonomy approval required",
			complexity:      models.TaskComplexityEasy,
			agentAutonomy:   models.AgentAutonomyApprovalRequired,
			wantSpecStatus:  models.TaskSpecStatusPendingReview,
			wantTaskStatus:  models.TaskStatusSpecReview,
		},
		{
			name:            "supervised easy task auto-approves",
			complexity:      models.TaskComplexityEasy,
			agentAutonomy:   models.AgentAutonomySupervised,
			wantSpecStatus:  models.TaskSpecStatusAutoApproved,
			wantTaskStatus:  models.TaskStatusCoding,
		},
		{
			name:            "supervised medium task requires review",
			complexity:      models.TaskComplexityMedium,
			agentAutonomy:   models.AgentAutonomySupervised,
			wantSpecStatus:  models.TaskSpecStatusPendingReview,
			wantTaskStatus:  models.TaskStatusSpecReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specStatus, taskStatus := ShouldAutoApproveSpec(
				tt.complexity,
				tt.affectedFiles,
				tt.riskDomains,
				tt.agentAutonomy,
				tt.projectAutonomy,
				tt.projectReviewPolicy,
				tt.hasClarifications,
			)
			if specStatus != tt.wantSpecStatus || taskStatus != tt.wantTaskStatus {
				t.Errorf("ShouldAutoApproveSpec() = (%q, %q), want (%q, %q)",
					specStatus, taskStatus, tt.wantSpecStatus, tt.wantTaskStatus)
			}
		})
	}
}
