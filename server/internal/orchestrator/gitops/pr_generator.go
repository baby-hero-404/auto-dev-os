package gitops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// PRGenerator builds AI-generated PR summaries and risk assessments.
type PRGenerator struct{}

func NewPRGenerator() *PRGenerator {
	return &PRGenerator{}
}

type FileDiff struct {
	Name    string
	Added   int
	Deleted int
}

func parseDiff(diffText string) []FileDiff {
	lines := strings.Split(diffText, "\n")
	var result []FileDiff
	var current *FileDiff

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				name := parts[2]
				name = strings.TrimPrefix(name, "a/")
				result = append(result, FileDiff{Name: name})
				current = &result[len(result)-1]
			}
		} else if current != nil {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				current.Added++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				current.Deleted++
			}
		}
	}
	return result
}

// GenerateSummary creates a structured PR summary from task data, agent info, changed files, git diff, and test results.
func (g *PRGenerator) GenerateSummary(
	ctx context.Context,
	task *models.Task,
	agent *models.Agent,
	changedFiles []string,
	diffText string,
	testResult map[string]any,
	riskDomains []string,
	reviewLimitExceeded bool,
	selfReviewFallback bool,
) *models.PRSummary {
	title := fmt.Sprintf("AutoCodeOS: %s", task.Title)

	// Build the PR body using the spec template.
	var body strings.Builder
	body.WriteString(fmt.Sprintf("## AutoCodeOS: %s\n\n", task.Title))
	body.WriteString(fmt.Sprintf("**Task:** #%s\n", task.ID))

	agentRole := "Unknown Agent"
	modelLevel := "unknown"
	if agent != nil {
		agentRole = agent.Role
		modelLevel = agent.ModelLevelGroup
	}
	body.WriteString(fmt.Sprintf("**Agent:** %s (%s)\n", agentRole, modelLevel))
	body.WriteString(fmt.Sprintf("**Complexity:** %s\n\n", task.Complexity))

	body.WriteString("### Summary\n")
	if task.Description != "" {
		body.WriteString(fmt.Sprintf("%s\n\n", task.Description))
	} else {
		body.WriteString("Automated changes implemented by Auto Code OS.\n\n")
	}

	body.WriteString("### Changes\n")
	fileDiffs := parseDiff(diffText)
	if len(fileDiffs) > 0 {
		body.WriteString("| File | Action | Lines Changed |\n")
		body.WriteString("|------|--------|---------------|\n")
		for _, fd := range fileDiffs {
			action := "Modified"
			if fd.Added > 0 && fd.Deleted == 0 {
				action = "Added"
			} else if fd.Deleted > 0 && fd.Added == 0 {
				action = "Deleted"
			}
			body.WriteString(fmt.Sprintf("| %s | %s | +%d / -%d |\n", fd.Name, action, fd.Added, fd.Deleted))
		}
	} else {
		// Fallback if diffText is not provided
		body.WriteString("| File | Action | Lines Changed |\n")
		body.WriteString("|------|--------|---------------|\n")
		for _, f := range changedFiles {
			body.WriteString(fmt.Sprintf("| %s | Modified | +/- |\n", f))
		}
	}
	body.WriteString("\n")

	// Risk assessment based on task complexity and file count.
	riskLevel, riskReason := assessRisk(task, changedFiles)
	body.WriteString("### Risk Assessment\n")
	body.WriteString(fmt.Sprintf("- **Level:** %s\n", riskLevel))
	if len(riskDomains) > 0 {
		body.WriteString(fmt.Sprintf("- **Risk Domains:** %s\n", strings.Join(riskDomains, ", ")))
	}
	body.WriteString(fmt.Sprintf("- **Reason:** %s\n", riskReason))
	if reviewLimitExceeded {
		body.WriteString("- ⚠️ **Review Limit Warning:** The review-fix cycle limit has been exceeded. Please review carefully.\n")
	}
	if selfReviewFallback {
		body.WriteString("- ⚠️ **Self-Review Warning:** No alternative model was available, so this PR was reviewed by the same model that wrote the code (Harness Independence fallback). Please review carefully.\n")
	}
	body.WriteString("\n")

	// Test Results
	body.WriteString("### Test Results\n")
	renderBullet := func(label string, status string) string {
		switch status {
		case "passed":
			return fmt.Sprintf("- ✅ %s: passed\n", label)
		case "failed":
			return fmt.Sprintf("- ❌ %s: failed\n", label)
		case "not_configured":
			return fmt.Sprintf("- ℹ️ %s: Not configured/no config found\n", label)
		default:
			return fmt.Sprintf("- ⚠️ %s: Not executed\n", label)
		}
	}

	if testResult != nil {
		backendStatus := "not_executed"
		if val, ok := testResult["targeted_code_backend_passed"]; ok {
			if passed, _ := val.(bool); passed {
				backendStatus = "passed"
			} else {
				backendStatus = "failed"
			}
		}
		frontendStatus := "not_executed"
		if val, ok := testResult["targeted_code_frontend_passed"]; ok {
			if passed, _ := val.(bool); passed {
				frontendStatus = "passed"
			} else {
				frontendStatus = "failed"
			}
		}

		postCodeStatus := "not_executed"
		if backendStatus != "not_executed" || frontendStatus != "not_executed" {
			postCodeStatus = "passed"
			if backendStatus == "failed" || frontendStatus == "failed" {
				postCodeStatus = "failed"
			}
		}
		body.WriteString(renderBullet("Targeted tests (post-code)", postCodeStatus))

		postFixStatus := "not_executed"
		if val, ok := testResult["targeted_fix_passed"]; ok {
			if passed, _ := val.(bool); passed {
				postFixStatus = "passed"
			} else {
				postFixStatus = "failed"
			}
		}
		body.WriteString(renderBullet("Targeted tests (post-fix)", postFixStatus))

		fullSuiteStatus := "not_executed"
		if passed, ok := testResult["passed"].(bool); ok {
			if passed {
				fullSuiteStatus = "passed"
			} else {
				fullSuiteStatus = "failed"
			}
		}
		body.WriteString(renderBullet("Full test suite", fullSuiteStatus))

		lintStatus := "not_executed"
		if val, ok := testResult["lint_status"].(string); ok {
			lintStatus = val
		} else if lintPassed, ok := testResult["lint_passed"].(bool); ok {
			if lintPassed {
				lintStatus = "passed"
			} else {
				lintStatus = "failed"
			}
		}
		body.WriteString(renderBullet("Lint", lintStatus))

		buildStatus := "not_executed"
		if val, ok := testResult["build_status"].(string); ok {
			buildStatus = val
		} else if buildPassed, ok := testResult["build_passed"].(bool); ok {
			if buildPassed {
				buildStatus = "passed"
			} else {
				buildStatus = "failed"
			}
		}
		body.WriteString(renderBullet("Build verification", buildStatus))
	} else {
		body.WriteString("- ⚠️ Targeted tests (post-code): Not executed\n")
		body.WriteString("- ⚠️ Targeted tests (post-fix): Not executed\n")
		body.WriteString("- ⚠️ Full test suite: Not executed\n")
		body.WriteString("- ⚠️ Lint: Not executed\n")
		body.WriteString("- ⚠️ Build verification: Not executed\n")
	}
	body.WriteString("\n---\n")
	body.WriteString("*This PR was generated by Auto Code OS.*\n")

	slog.Info("pr summary generated", "task_id", task.ID, "files", len(changedFiles), "risk", riskLevel)

	return &models.PRSummary{
		Title:               title,
		Body:                body.String(),
		ChangedFiles:        changedFiles,
		RiskLevel:           riskLevel,
		RiskReason:          riskReason,
		RiskDomains:         riskDomains,
		ReviewLimitExceeded: reviewLimitExceeded,
		SelfReviewFallback:  selfReviewFallback,
		Status:              models.PRStatusOpen,
	}
}

// assessRisk evaluates the risk level of a PR based on task complexity and scope.
func assessRisk(task *models.Task, changedFiles []string) (string, string) {
	fileCount := len(changedFiles)

	// Check for critical file patterns.
	hasMigration := false
	hasConfig := false
	for _, f := range changedFiles {
		if strings.Contains(f, "migration/") {
			hasMigration = true
		}
		if strings.Contains(f, "config") || strings.Contains(f, ".env") || strings.Contains(f, "docker") {
			hasConfig = true
		}
	}

	switch {
	case hasMigration && task.Complexity == models.TaskComplexityHard:
		return models.PRRiskCritical, "Database migration in a hard-complexity task requires careful review"
	case hasMigration:
		return models.PRRiskHigh, "Contains database migration files"
	case task.Complexity == models.TaskComplexityHard || fileCount > 15:
		return models.PRRiskHigh, fmt.Sprintf("Hard complexity task affecting %d files", fileCount)
	case hasConfig:
		return models.PRRiskMedium, "Modifies configuration or infrastructure files"
	case task.Complexity == models.TaskComplexityMedium || fileCount > 5:
		return models.PRRiskMedium, fmt.Sprintf("Medium complexity task affecting %d files", fileCount)
	default:
		return models.PRRiskLow, fmt.Sprintf("Simple change affecting %d files", fileCount)
	}
}
