package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func GetTaskWorkspace(workspaceRoot string, task *models.Task) *models.TaskWorkspace {
	root := sandbox.WorkspacePath(workspaceRoot, task.ID)
	return &models.TaskWorkspace{
		Root:         root,
		SpecsDir:     filepath.Join(root, "specs"),
		ContextDir:   filepath.Join(root, "context"),
		ArtifactsDir: filepath.Join(root, "artifacts"),
		LogsDir:      filepath.Join(root, "logs"),
		PRDir:        filepath.Join(root, "pr"),
	}
}

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

func QuoteShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

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

func IsSafeRelativeSourcePath(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) {
		return false
	}
	return path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator))
}
