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
