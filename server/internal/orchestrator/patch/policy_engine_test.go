package patch

import (
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestEvaluatePolicy(t *testing.T) {
	analysis := &models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "repository",
				Root:         "internal/repository",
				Capabilities: []string{"modify_existing", "create_test"},
			},
			{
				Module:       "sqlite",
				Root:         "internal/sqlite",
				RepoName:     "tool_zentao",
				Capabilities: []string{"modify_existing", "create_test"},
			},
		},
	}

	tests := []struct {
		name                string
		file                string
		oldFile             string
		isRepoRootWorkspace bool
		expectedSev         Severity
		expectedReason      string
	}{
		{
			name:                "Critical file - Dockerfile",
			file:                "Dockerfile",
			oldFile:             "Dockerfile",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityCritical,
		},
		{
			name:                "Critical file - .github/workflows/ci.yml",
			file:                ".github/workflows/ci.yml",
			oldFile:             "",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityCritical,
		},
		{
			name:                "Common config - go.mod without capability",
			file:                "go.mod",
			oldFile:             "go.mod",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityWarning, // Auto-expand with warning
		},
		{
			name:                "Valid change - existing sqlite.go in module root",
			file:                "internal/repository/sqlite.go",
			oldFile:             "internal/repository/sqlite.go",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityInfo, // Allowed under modify_existing
		},
		{
			name:                "Valid change - new sqlite_test.go (test capability)",
			file:                "internal/repository/sqlite_test.go",
			oldFile:             "",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityInfo, // Allowed under create_test
		},
		{
			name:                "Auto-expand helper - new helper.go in module root (missing create_helper)",
			file:                "internal/repository/helper.go",
			oldFile:             "",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityInfo, // Auto-expand low risk
		},
		{
			name:                "Error - file outside boundary",
			file:                "internal/service/auth.go",
			oldFile:             "internal/service/auth.go",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityError, // Soft retry
		},
		{
			name:                "Valid change under repo with repo prefix in file path",
			file:                "tool_zentao/internal/sqlite/repository.go",
			oldFile:             "tool_zentao/internal/sqlite/repository.go",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityInfo,
		},
		{
			name:                "Valid change under repo without repo prefix in file path (raw match)",
			file:                "internal/sqlite/repository.go",
			oldFile:             "internal/sqlite/repository.go",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityInfo,
		},
		{
			name:                "Error - self-nested path starting with code/repos, repo-root workspace",
			file:                "code/repos/tool_zentao/main/cmd/sync/main.go",
			oldFile:             "",
			isRepoRootWorkspace: true,
			expectedSev:         SeverityError,
		},
		{
			// Regression (Part 2 review): a multi-repo task's tool workspace is the flat
			// CodeRoot, so "code/repos/<repo>/..." is the CORRECT relative path form there —
			// the self-nesting guard must not fire when isRepoRootWorkspace is false.
			name:                "Not an error - same path is correct for a multi-repo (non-repo-root) workspace",
			file:                "code/repos/repoB/main/utils.go",
			oldFile:             "",
			isRepoRootWorkspace: false,
			expectedSev:         SeverityError, // still boundary-scoped by analysis (no matching boundary) — not the nesting guard
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := EvaluatePolicy(tc.file, tc.oldFile, analysis, tc.isRepoRootWorkspace)
			if dec.Severity != tc.expectedSev {
				t.Fatalf("expected severity %v, got %v (reason: %q)", tc.expectedSev, dec.Severity, dec.Reason)
			}
			if !tc.isRepoRootWorkspace && strings.Contains(dec.Reason, "appears workspace-prefixed") {
				t.Errorf("self-nesting guard fired despite isRepoRootWorkspace=false, reason: %q", dec.Reason)
			}
		})
	}
}

func TestEvaluatePolicyInfrastructure(t *testing.T) {
	analysis := &models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "makefile_specific",
				Root:         "Makefile",
				Capabilities: []string{"modify_existing"},
			},
			{
				Module:       "root_infra",
				Root:         ".",
				Capabilities: []string{"modify_infrastructure", "modify_existing"},
			},
		},
	}

	// 1. Specific boundary "Makefile" with modify_existing should be allowed.
	dec1 := EvaluatePolicy("Makefile", "Makefile", analysis, true)
	if dec1.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for Makefile with specific root and modify_existing, got %s (reason: %q)", dec1.Severity, dec1.Reason)
	}

	// 2. Dockerfile covered by root "." which has "modify_infrastructure" boundary.
	dec2 := EvaluatePolicy("Dockerfile", "Dockerfile", analysis, true)
	if dec2.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for Dockerfile with modify_infrastructure, got %s (reason: %q)", dec2.Severity, dec2.Reason)
	}

	// 3. Dockerfile covered by an analysis WITHOUT modify_infrastructure should be blocked.
	analysisBlocked := &models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "general_root",
				Root:         ".",
				Capabilities: []string{"modify_existing"},
			},
		},
	}
	dec3 := EvaluatePolicy("Dockerfile", "Dockerfile", analysisBlocked, true)
	if dec3.Severity != SeverityCritical {
		t.Errorf("expected SeverityCritical for Dockerfile under general root without modify_infrastructure, got %s (reason: %q)", dec3.Severity, dec3.Reason)
	}

	// 4. Dockerfile listed in AffectedFiles under an analysis without modify_infrastructure should be allowed
	// as it is explicitly pre-approved by the planning analysis phase.
	analysisPreApproved := &models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{
			{File: "Dockerfile"},
		},
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "general_root",
				Root:         ".",
				Capabilities: []string{"modify_existing"},
			},
		},
	}
	dec4 := EvaluatePolicy("Dockerfile", "Dockerfile", analysisPreApproved, true)
	if dec4.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for Dockerfile pre-approved in AffectedFiles, got %s (reason: %q)", dec4.Severity, dec4.Reason)
	}
}

func TestEvaluatePolicyDependencyConfig(t *testing.T) {
	// 1. Dependency config WITHOUT pre-approval or explicit capability -> SeverityWarning
	analysisWarning := &models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "general",
				Root:         ".",
				Capabilities: []string{"modify_existing"},
			},
		},
	}
	dec1 := EvaluatePolicy("package.json", "package.json", analysisWarning, true)
	if dec1.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for unplanned package.json, got %s (reason: %q)", dec1.Severity, dec1.Reason)
	}

	// 2. Dependency config WITH pre-approval in AffectedFiles -> SeverityInfo
	analysisPreApproved := &models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{
			{File: "package.json"},
		},
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "general",
				Root:         ".",
				Capabilities: []string{"modify_existing"},
			},
		},
	}
	dec2 := EvaluatePolicy("package.json", "package.json", analysisPreApproved, true)
	if dec2.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for pre-approved package.json, got %s (reason: %q)", dec2.Severity, dec2.Reason)
	}
}

func TestEvaluatePolicyOutOfBoundaryFallback(t *testing.T) {
	// 1. Out-of-boundary file WITHOUT pre-approval -> SeverityError (Soft Retry Loop)
	analysisError := &models.TaskAnalysis{
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "src",
				Root:         "src",
				Capabilities: []string{"modify_existing"},
			},
		},
	}
	dec1 := EvaluatePolicy("unrelated/file.go", "unrelated/file.go", analysisError, true)
	if dec1.Severity != SeverityError {
		t.Errorf("expected SeverityError for completely out-of-bounds file, got %s (reason: %q)", dec1.Severity, dec1.Reason)
	}

	// 2. Out-of-boundary file WITH pre-approval -> SeverityWarning (Graceful Degradation)
	analysisFallback := &models.TaskAnalysis{
		AffectedFiles: []models.AffectedFile{
			{File: "unrelated/file.go"},
		},
		ExecutionBoundaries: []models.ExecutionBoundary{
			{
				Module:       "src",
				Root:         "src",
				Capabilities: []string{"modify_existing"},
			},
		},
	}
	dec2 := EvaluatePolicy("unrelated/file.go", "unrelated/file.go", analysisFallback, true)
	if dec2.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for pre-approved but out-of-bounds file, got %s (reason: %q)", dec2.Severity, dec2.Reason)
	}
}
