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

// EvaluatePolicy scores a proposed file change against the task's execution boundaries.
// isRepoRootWorkspace must be true only when the caller's tool workspace is itself a single
// repository checkout root (see tool.IsRepoCheckoutWorkspace) — for multi-repo tasks the tool
// workspace is the flat CodeRoot, where "code/repos/<repo>/..." is the CORRECT relative path
// form, not the self-nesting bug pattern the check below guards against.
func EvaluatePolicy(file string, currentOldFile string, analysis *models.TaskAnalysis, isRepoRootWorkspace bool) PolicyDecision {
	file = filepath.ToSlash(filepath.Clean(strings.TrimSpace(file)))
	isNew := currentOldFile == "/dev/null" || currentOldFile == ""
	baseName := filepath.Base(file)

	// Self-nested path check — only meaningful when the workspace root IS a repo checkout.
	if isRepoRootWorkspace && strings.HasPrefix(strings.TrimPrefix(file, "/"), "code/repos/") {
		return PolicyDecision{
			Severity: SeverityError,
			Reason:   fmt.Sprintf("path %q appears workspace-prefixed; this workspace is the repository root — use a repository-relative path instead", file),
			Risk:     "HIGH",
		}
	}

	// 1. Critical infrastructure / security check
	isCritical := false
	for _, pat := range criticalPatterns {
		if pat.MatchString(file) {
			isCritical = true
			break
		}
	}

	if isCritical {
		// Check if there is an execution boundary that explicitly authorizes modifying infrastructure files.
		hasInfraCap := false
		if analysis != nil {
			// Check if the file is explicitly estimated/pre-approved in the task's planned AffectedFiles
			isPreApprovedInSpec := false
			for _, af := range analysis.AffectedFiles {
				afFile := filepath.ToSlash(filepath.Clean(strings.TrimSpace(af.File)))
				if file == afFile || strings.HasSuffix(file, "/"+afFile) || strings.HasSuffix(afFile, "/"+file) {
					isPreApprovedInSpec = true
					break
				}
			}

			for _, b := range analysis.ExecutionBoundaries {
				cleanRoot := filepath.ToSlash(filepath.Clean(b.Root))
				if cleanRoot == "." || cleanRoot == "/" {
					cleanRoot = ""
				}
				isMatch := false
				if cleanRoot == "" {
					isMatch = true
				} else if file == cleanRoot || strings.HasPrefix(file, cleanRoot+"/") {
					isMatch = true
				}
				if !isMatch && b.RepoName != "" {
					prefix := b.RepoName + "/"
					if strings.HasPrefix(file, prefix) {
						stripped := strings.TrimPrefix(file, prefix)
						if cleanRoot == "" || stripped == cleanRoot || strings.HasPrefix(stripped, cleanRoot+"/") {
							isMatch = true
						}
					} else if file == b.RepoName && cleanRoot == "" {
						isMatch = true
					}
				}
				if !isMatch && b.RepoName != "" && cleanRoot != "" {
					repoRoot := b.RepoName + "/" + cleanRoot
					if file == repoRoot || strings.HasPrefix(file, repoRoot+"/") {
						isMatch = true
					}
				}

				if isMatch {
					if isPreApprovedInSpec {
						hasInfraCap = true
						break
					}
					// We allow infrastructure edits under two conditions:
					// 1. The boundary explicitly grants "modify_infrastructure" or "modify_sensitive" capability.
					// 2. The boundary has a specific root (not general repository root like "." or "")
					//    and grants "modify_existing" or "create_helper".
					isSpecificRoot := cleanRoot != "" && cleanRoot != "." && cleanRoot != "/"
					for _, cap := range b.Capabilities {
						if cap == "modify_infrastructure" || cap == "modify_sensitive" {
							hasInfraCap = true
							break
						}
						if isSpecificRoot && (cap == "modify_existing" || cap == "create_helper") {
							hasInfraCap = true
							break
						}
					}
				}
				if hasInfraCap {
					break
				}
			}
		}

		if !hasInfraCap {
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

		// If not explicitly approved, check if pre-approved in spec!
		if analysis != nil {
			for _, af := range analysis.AffectedFiles {
				afFile := filepath.ToSlash(filepath.Clean(strings.TrimSpace(af.File)))
				if file == afFile || strings.HasSuffix(file, "/"+afFile) || strings.HasSuffix(afFile, "/"+file) {
					return PolicyDecision{
						Severity:   SeverityInfo,
						Reason:     fmt.Sprintf("dependency config file pre-approved in planning spec: %q", file),
						Capability: "add_dependency",
						Risk:       "MEDIUM",
					}
				}
			}
		}

		// If not explicitly approved and not in spec, we auto-expand with warning
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
		if cleanRoot == "." || cleanRoot == "/" {
			cleanRoot = ""
		}

		isMatch := false
		if cleanRoot == "" {
			isMatch = true
		} else if file == cleanRoot || strings.HasPrefix(file, cleanRoot+"/") {
			isMatch = true
		}

		// Check match by stripping RepoName prefix (e.g. "tool_zentao/" from "tool_zentao/internal/sqlite/repository.go")
		if !isMatch && b.RepoName != "" {
			prefix := b.RepoName + "/"
			if strings.HasPrefix(file, prefix) {
				stripped := strings.TrimPrefix(file, prefix)
				if cleanRoot == "" || stripped == cleanRoot || strings.HasPrefix(stripped, cleanRoot+"/") {
					isMatch = true
				}
			} else if file == b.RepoName && cleanRoot == "" {
				isMatch = true
			}
		}

		// Check match by prepending RepoName to cleanRoot (e.g. comparing "tool_zentao/internal/sqlite" with file)
		if !isMatch && b.RepoName != "" && cleanRoot != "" {
			repoRoot := b.RepoName + "/" + cleanRoot
			if file == repoRoot || strings.HasPrefix(file, repoRoot+"/") {
				isMatch = true
			}
		}

		if isMatch {
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

	// 4. Outside of all approved boundaries -> check pre-approval bypass!
	if analysis != nil {
		for _, af := range analysis.AffectedFiles {
			afFile := filepath.ToSlash(filepath.Clean(strings.TrimSpace(af.File)))
			if file == afFile || strings.HasSuffix(file, "/"+afFile) || strings.HasSuffix(afFile, "/"+file) {
				return PolicyDecision{
					Severity:   SeverityWarning,
					Reason:     fmt.Sprintf("file %q is outside execution boundaries but pre-approved in planning spec; auto-expanding boundary", file),
					Capability: requiredCap,
					Risk:       "MEDIUM",
				}
			}
		}
	}

	// 5. Outside of all approved boundaries and not pre-approved -> Soft Retry (SeverityError)
	return PolicyDecision{
		Severity:   SeverityError,
		Reason:     fmt.Sprintf("file %q is outside of all approved execution boundaries", file),
		Capability: requiredCap,
		Risk:       "HIGH",
	}
}
