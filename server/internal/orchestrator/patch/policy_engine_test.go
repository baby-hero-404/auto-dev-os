package patch

import (
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
		},
	}

	tests := []struct {
		name           string
		file           string
		oldFile        string
		expectedSev    Severity
		expectedReason string
	}{
		{
			name:        "Critical file - Dockerfile",
			file:        "Dockerfile",
			oldFile:     "Dockerfile",
			expectedSev: SeverityCritical,
		},
		{
			name:        "Critical file - .github/workflows/ci.yml",
			file:        ".github/workflows/ci.yml",
			oldFile:     "",
			expectedSev: SeverityCritical,
		},
		{
			name:        "Common config - go.mod without capability",
			file:        "go.mod",
			oldFile:     "go.mod",
			expectedSev: SeverityWarning, // Auto-expand with warning
		},
		{
			name:        "Valid change - existing sqlite.go in module root",
			file:        "internal/repository/sqlite.go",
			oldFile:     "internal/repository/sqlite.go",
			expectedSev: SeverityInfo, // Allowed under modify_existing
		},
		{
			name:        "Valid change - new sqlite_test.go (test capability)",
			file:        "internal/repository/sqlite_test.go",
			oldFile:     "",
			expectedSev: SeverityInfo, // Allowed under create_test
		},
		{
			name:        "Auto-expand helper - new helper.go in module root (missing create_helper)",
			file:        "internal/repository/helper.go",
			oldFile:     "",
			expectedSev: SeverityInfo, // Auto-expand low risk
		},
		{
			name:        "Error - file outside boundary",
			file:        "internal/service/auth.go",
			oldFile:     "internal/service/auth.go",
			expectedSev: SeverityError, // Soft retry
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec := EvaluatePolicy(tc.file, tc.oldFile, analysis)
			if dec.Severity != tc.expectedSev {
				t.Fatalf("expected severity %v, got %v (reason: %q)", tc.expectedSev, dec.Severity, dec.Reason)
			}
		})
	}
}
