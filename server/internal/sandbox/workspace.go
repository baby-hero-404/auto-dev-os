package sandbox

import (
	"path/filepath"
)

func WorkspacePath(root, taskID string) string {
	if root == "" {
		root = "/tmp/auto-code-os/workspaces"
	}
	return filepath.Join(root, taskID)
}
