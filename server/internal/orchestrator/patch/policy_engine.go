package patch

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"     // Auto-expand + Log (Recoverable)
	SeverityWarning  Severity = "WARNING"  // Auto-expand + Warning log
	SeverityError    Severity = "ERROR"    // Soft Retry (Correctable)
	SeverityCritical Severity = "CRITICAL" // Hard Fail + Pause (Critical)
)

type PolicyViolationError struct {
	Severity    Severity `json:"severity"`
	ErrorMsg    string   `json:"error"`
	Reason      string   `json:"reason"`
	AllowedRoot string   `json:"allowed_root"`
	Violations  []string `json:"violations"`
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("policy_violation (%s): %s", e.Severity, e.ErrorMsg)
}

type PolicyDecision struct {
	Severity   Severity
	Reason     string
	Capability string
	Risk       string // LOW, MEDIUM, HIGH, CRITICAL
}

var criticalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\.github/`),
	regexp.MustCompile(`(?i)Dockerfile`),
	regexp.MustCompile(`(?i)Makefile`),
	regexp.MustCompile(`(?i)docker-compose`),
	regexp.MustCompile(`^terraform/`),
	regexp.MustCompile(`^scripts/`),
	regexp.MustCompile(`^\.env`),
	regexp.MustCompile(`(?i)credential`),
	regexp.MustCompile(`(?i)secret`),
}

func EvaluatePolicy(file string, currentOldFile string, analysis *models.TaskAnalysis) PolicyDecision {
	file = filepath.ToSlash(filepath.Clean(strings.TrimSpace(file)))
	baseName := filepath.Base(file)
	isNew := currentOldFile == "/dev/null" || currentOldFile == ""

	// 1. Critical infrastructure / security check
	for _, pat := range criticalPatterns {
		if pat.MatchString(file) {
			return PolicyDecision{
				Severity: SeverityCritical,
				Reason:   fmt.Sprintf("modification to infrastructure/security-sensitive file: %q", file),
				Risk:     "CRITICAL",
			}
		}
	}

	// 2. Common config / dependency check
	isConfig := false
	switch baseName {
	case "go.mod", "go.sum", "package.json", "yarn.lock", "package-lock.json", "pnpm-lock.yaml", "Cargo.toml", "Cargo.lock", "requirements.txt", "poetry.lock", "pyproject.toml":
		isConfig = true
	}

	if isConfig {
		// Check if any boundary allows add_dependency
		hasDepCap := false
		for _, b := range analysis.ExecutionBoundaries {
			for _, cap := range b.Capabilities {
				if cap == "add_dependency" {
					hasDepCap = true
					break
				}
			}
		}
		if hasDepCap {
			return PolicyDecision{
				Severity:   SeverityInfo,
				Reason:     fmt.Sprintf("dependency config file approved by capability: %q", file),
				Capability: "add_dependency",
				Risk:       "MEDIUM",
			}
		}
		// If not explicitly approved, we auto-expand with warning
		return PolicyDecision{
			Severity:   SeverityWarning,
			Reason:     fmt.Sprintf("auto-expanding boundary for dependency config file: %q", file),
			Capability: "add_dependency",
			Risk:       "MEDIUM",
		}
	}

	// 3. Match against Execution Boundaries (Blast Radius)
	var matchedBoundary *models.ExecutionBoundary
	for _, b := range analysis.ExecutionBoundaries {
		cleanRoot := filepath.ToSlash(filepath.Clean(b.Root))
		// Root boundary match
		if file == cleanRoot || strings.HasPrefix(file, cleanRoot+"/") {
			matchedBoundary = &b
			break
		}
	}

	// Determine required capability
	requiredCap := "modify_existing"
	isTest := strings.HasSuffix(baseName, "_test.go") || strings.Contains(baseName, ".test.") || strings.Contains(baseName, ".spec.")
	isMock := strings.Contains(strings.ToLower(baseName), "mock")
	isExport := baseName == "index.ts" || baseName == "mod.rs" || baseName == "__init__.py"

	if isTest {
		requiredCap = "create_test"
	} else if isMock {
		requiredCap = "generate_mock"
	} else if isExport {
		requiredCap = "modify_exports"
	} else if isNew {
		requiredCap = "create_helper"
	}

	if matchedBoundary != nil {
		// Same module root! Check if capability is present
		hasCap := false
		for _, cap := range matchedBoundary.Capabilities {
			if cap == requiredCap {
				hasCap = true
				break
			}
		}

		if hasCap {
			return PolicyDecision{
				Severity:   SeverityInfo,
				Reason:     fmt.Sprintf("file change authorized under module %q with capability %q", matchedBoundary.Module, requiredCap),
				Capability: requiredCap,
				Risk:       "LOW",
			}
		}

		// Same module, but missing capability.
		// If creating tests or helpers, it is low risk. Auto-expand!
		if requiredCap == "create_test" || requiredCap == "create_helper" || requiredCap == "generate_mock" {
			return PolicyDecision{
				Severity:   SeverityInfo,
				Reason:     fmt.Sprintf("auto-expanding capability %q under module %q for file: %q", requiredCap, matchedBoundary.Module, file),
				Capability: requiredCap,
				Risk:       "LOW",
			}
		}

		// Other capabilities inside same module: Warning auto-expand
		return PolicyDecision{
			Severity:   SeverityWarning,
			Reason:     fmt.Sprintf("missing capability %q under module %q for file: %q. Auto-expanding with warning.", requiredCap, matchedBoundary.Module, file),
			Capability: requiredCap,
			Risk:       "LOW",
		}
	}

	// 4. Outside of all approved boundaries -> Soft Retry (SeverityError)
	return PolicyDecision{
		Severity:   SeverityError,
		Reason:     fmt.Sprintf("file %q is outside of all approved execution boundaries", file),
		Capability: requiredCap,
		Risk:       "HIGH",
	}
}
