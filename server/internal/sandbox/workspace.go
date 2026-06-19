package sandbox

import (
	"path/filepath"
)

func WorkspacePath(root, taskID string) string {
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	absRoot, err := filepath.Abs(root)
	if err == nil {
		root = absRoot
	}
	return filepath.Join(root, taskID)
}
