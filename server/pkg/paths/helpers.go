package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContainerPathForHostPath resolves container path from host path.
func ContainerPathForHostPath(localPath string, activeWorkspaceHostPath string, hostPath string) string {
	relMain, errMain := filepath.Rel(localPath, hostPath)
	if errMain == nil && relMain != ".." && !strings.HasPrefix(relMain, ".."+string(filepath.Separator)) {
		if relMain == "." {
			return "/workspace"
		}
		return filepath.Join("/workspace", relMain)
	}

	rel, err := filepath.Rel(activeWorkspaceHostPath, hostPath)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if rel == "." {
			return "/workspace"
		}
		return filepath.Join("/workspace", rel)
	}

	return "/workspace"
}

// QuoteShellArg quotes a shell argument to make it safe for bash executions.
func QuoteShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ReadLimitedFile reads file with limited byte size.
func ReadLimitedFile(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	limit := maxBytes
	if stat.Size() < limit {
		limit = stat.Size()
	}

	buf := make([]byte, limit)
	n, errRead := file.Read(buf)
	if errRead != nil && n == 0 {
		return "", errRead
	}

	// Check if file is binary by searching for null bytes
	for i := 0; i < n; i++ {
		if buf[i] == 0x00 {
			return "[BINARY FILE: cannot display content]", nil
		}
	}

	content := string(buf[:n])
	if stat.Size() > maxBytes {
		content += "\n[TRUNCATED: file exceeded size limit]"
	}
	return content, nil
}

// ResolveSafePath resolves a safe path preventing directory traversal.
func ResolveSafePath(root, subPath string) (string, error) {
	absRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		absRoot, err = filepath.Abs(root)
		if err != nil {
			return "", err
		}
	}
	absRoot = filepath.Clean(absRoot)

	target := filepath.Join(absRoot, subPath)
	absTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		absTarget, err = filepath.Abs(target)
		if err != nil {
			return "", err
		}
	}
	absTarget = filepath.Clean(absTarget)

	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	return absTarget, nil
}

// IsSafeRelativeSourcePath checks if path is safe and relative.
func IsSafeRelativeSourcePath(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) {
		return false
	}
	return path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator))
}

// CanonicalizeRepoRelative normalizes a path that may carry workspace
// prefixes into a clean repository-relative path.
//
//	"code/repos/tool_zentao/main/cmd/sync/main.go" → "cmd/sync/main.go"
//	"cmd/sync/main.go"                             → "cmd/sync/main.go"
//	"code/repos/x/main/code/repos/x/main/a.go"     → "a.go" (collapses duplicates)
//
// Returns ok=false when the path escapes the repo or still contains a
// foreign repo prefix after stripping (caller drops the finding + warns).
func CanonicalizeRepoRelative(p, repoName, branch string) (string, bool) {
	if p == "" {
		return "", false
	}
	// Normalize separators
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/")

	// Reject directory traversal escapes in the raw components
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", false
		}
	}

	p = filepath.ToSlash(filepath.Clean(p))

	// Repeatedly strip workspace/repo prefixes
	for {
		orig := p

		if strings.HasPrefix(p, "code/repos/") {
			if repoName == "" || !strings.HasPrefix(p, "code/repos/"+repoName+"/") {
				return "", false
			}
			p = p[len("code/repos/"):]
		}

		if strings.HasPrefix(p, "a/") {
			p = p[2:]
		} else if strings.HasPrefix(p, "b/") {
			p = p[2:]
		}

		if repoName != "" {
			if strings.HasPrefix(p, repoName+"/") {
				p = p[len(repoName)+1:]
				// Check for worktrees/<role>/
				if strings.HasPrefix(p, "worktrees/") {
					parts := strings.SplitN(p, "/", 3)
					if len(parts) >= 3 {
						p = parts[2]
					}
				}
				// Check for branch name or "main"
				if branch != "" && strings.HasPrefix(p, branch+"/") {
					p = p[len(branch)+1:]
				} else if strings.HasPrefix(p, "main/") {
					p = p[len("main")+1:]
				}
			}
		}

		if p == orig {
			break
		}
	}

	// Double check that we did not leave any foreign repo prefix or traversal
	if strings.Contains(p, "code/repos/") || strings.Contains(p, "worktrees/") {
		return "", false
	}

	if p == "" || p == "." {
		return "", false
	}

	return p, true
}

// DeriveBranchName generates a clean, URL/branch-safe branch name from task ID and title.
// Format: feature/<slugified_title>-<short_task_id>
func DeriveBranchName(taskID string, title string) string {
	slug := Slugify(title)
	shortID := taskID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if slug == "" {
		return "feature/" + shortID
	}
	return "feature/" + slug + "-" + shortID
}

// DeriveTaskSlug generates the same slug-<short_task_id> identifier used by
// DeriveBranchName but without the "feature/" branch prefix, for use as a
// directory name (e.g. docs/openspecs/<task-slug>/).
func DeriveTaskSlug(taskID string, title string) string {
	slug := Slugify(title)
	shortID := taskID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if slug == "" {
		return shortID
	}
	return slug + "-" + shortID
}

// DeriveRoleBranchName generates a clean, URL/branch-safe branch name with a role suffix.
func DeriveRoleBranchName(taskID string, title string, roleSuffix string) string {
	branch := DeriveBranchName(taskID, title)
	if roleSuffix == "" {
		return branch
	}
	return branch + "-" + roleSuffix
}

// Slugify converts a string into a clean lowercase slug (dashes and alphanumeric characters only).
func Slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastWasDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastWasDash = false
		} else {
			if !lastWasDash {
				b.WriteRune('-')
				lastWasDash = true
			}
		}
	}
	res := b.String()
	res = strings.Trim(res, "-")
	if len(res) > 30 {
		res = res[:30]
	}
	return strings.Trim(res, "-")
}


