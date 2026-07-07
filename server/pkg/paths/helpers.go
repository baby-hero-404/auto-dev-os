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
