package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsRepoCheckoutWorkspace reports whether workspace is a single repository checkout root
// (e.g. .../code/repos/<repo>/main or a directory containing .git), as opposed to the flat
// multi-repo CodeRoot (.../code) used when a task spans more than one repository. Multi-repo
// tasks legitimately use "code/repos/<repo>/..." as a relative path — the self-nesting guards
// in this file and patch.EvaluatePolicy must only fire when workspace is itself a repo root,
// or they misclassify correct multi-repo paths as the nested-path bug (task 8291a25e report,
// Part 2 review finding on EvaluatePolicy).
func IsRepoCheckoutWorkspace(workspace string) bool {
	if strings.Contains(filepath.ToSlash(workspace), "/code/repos/") {
		return true
	}
	_, err := os.Stat(filepath.Join(workspace, ".git"))
	return err == nil
}

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

	// Tool-Layer Guard: reject code/repos/... prefixed relPaths when the workspace is a repository root
	if IsRepoCheckoutWorkspace(absWorkspace) {
		normalizedRel := strings.ReplaceAll(relPath, "\\", "/")
		normalizedRel = strings.TrimPrefix(normalizedRel, "/")
		if strings.HasPrefix(normalizedRel, "code/repos/") {
			parts := strings.Split(normalizedRel, "/")
			correctPath := normalizedRel
			if len(parts) >= 5 && parts[0] == "code" && parts[1] == "repos" {
				if parts[3] == "worktrees" && len(parts) >= 6 {
					correctPath = strings.Join(parts[5:], "/")
				} else {
					correctPath = strings.Join(parts[4:], "/")
				}
			}
			return "", fmt.Errorf("path %q appears workspace-prefixed; this workspace is the repository root — use %q", relPath, correctPath)
		}
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
