package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// SafeWorkspacePath resolves relPath within the workspace and prevents directory traversal.
func SafeWorkspacePath(workspace string, relPath string) (string, error) {
	if workspace == "" {
		return "", fmt.Errorf("workspace root is empty")
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("invalid workspace root: %w", err)
	}

	var resolved string
	if filepath.IsAbs(relPath) {
		resolved = filepath.Clean(relPath)
	} else {
		resolved = filepath.Join(absWorkspace, relPath)
	}

	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Ensure prefix matching with separator to prevent path tricks like "/workspace-other" matching "/workspace"
	sep := string(filepath.Separator)
	prefix := absWorkspace
	if !strings.HasSuffix(prefix, sep) {
		prefix += sep
	}

	if !strings.HasPrefix(absResolved+sep, prefix) {
		return "", fmt.Errorf("security violation: path %q escapes workspace boundary", relPath)
	}

	return absResolved, nil
}

// Sha256Hash returns the first 8 characters of the sha256 hash of data.
func Sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}

// GenerateDiffPreview constructs a simple diff block showing search and replace changes.
func GenerateDiffPreview(search, replace, filePath string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	builder.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))
	builder.WriteString("@@ -1 +1 @@\n")

	searchLines := strings.Split(search, "\n")
	for _, l := range searchLines {
		builder.WriteString(fmt.Sprintf("-%s\n", l))
	}
	replaceLines := strings.Split(replace, "\n")
	for _, l := range replaceLines {
		builder.WriteString(fmt.Sprintf("+%s\n", l))
	}
	return builder.String()
}
