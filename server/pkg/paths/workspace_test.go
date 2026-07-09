package paths

import (
	"testing"
)

func TestWorkspaceToRepoRelative(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Main branch file absolute path",
			path:     "/home/ubuntu/workspace/code/repos/tool_zentao/main/go.mod",
			expected: "tool_zentao/go.mod",
		},
		{
			name:     "Main branch folder absolute path",
			path:     "/home/ubuntu/workspace/code/repos/tool_zentao/main",
			expected: "tool_zentao",
		},
		{
			name:     "Worktree file absolute path",
			path:     "/home/ubuntu/workspace/code/repos/tool_zentao/worktrees/backend/internal/config/config.go",
			expected: "tool_zentao/internal/config/config.go",
		},
		{
			name:     "Worktree folder absolute path",
			path:     "/home/ubuntu/workspace/code/repos/tool_zentao/worktrees/backend",
			expected: "tool_zentao",
		},
		{
			name:     "Workspace relative repo root path",
			path:     "code/repos/tool_zentao",
			expected: "tool_zentao",
		},
		{
			name:     "Unrelated path returns same",
			path:     "some/other/path/file.txt",
			expected: "some/other/path/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := WorkspaceToRepoRelative(tt.path)
			if actual != tt.expected {
				t.Errorf("WorkspaceToRepoRelative(%q) = %q, expected %q", tt.path, actual, tt.expected)
			}
		})
	}
}
