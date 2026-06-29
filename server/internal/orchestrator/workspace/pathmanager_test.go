package workspace

import (
	"testing"
)

func TestWorkspaceToRepoRelative(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard main path",
			input:    "code/repos/test/main/readme.md",
			expected: "test/readme.md",
		},
		{
			name:     "nested path under main",
			input:    "code/repos/test/main/numbers/utils.go",
			expected: "test/numbers/utils.go",
		},
		{
			name:     "worktree path",
			input:    "code/repos/test/worktrees/backend/src/main.go",
			expected: "test/src/main.go",
		},
		{
			name:     "already repo-relative",
			input:    "readme.md",
			expected: "readme.md",
		},
		{
			name:     "non-matching prefix",
			input:    "openspec/changes/readme/proposal.md",
			expected: "openspec/changes/readme/proposal.md",
		},
		{
			name:     "path too shallow to strip",
			input:    "code/repos/test",
			expected: "code/repos/test",
		},
		{
			name:     "path with repo name only plus main",
			input:    "code/repos/test/main",
			expected: "code/repos/test/main",
		},
		{
			name:     "multi-segment repo name path",
			input:    "code/repos/my-app/main/src/components/Button.tsx",
			expected: "my-app/src/components/Button.tsx",
		},
		{
			name:     "backslash normalisation",
			input:    "code\\repos\\test\\main\\readme.md",
			expected: "test/readme.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorkspaceToRepoRelative(tt.input)
			if got != tt.expected {
				t.Errorf("WorkspaceToRepoRelative(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRepoRelativeToWorkspace(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		branch   string
		path     string
		expected string
	}{
		{
			name:     "simple file",
			repo:     "test",
			branch:   "main",
			path:     "readme.md",
			expected: "code/repos/test/main/readme.md",
		},
		{
			name:     "nested file",
			repo:     "test",
			branch:   "main",
			path:     "numbers/utils.go",
			expected: "code/repos/test/main/numbers/utils.go",
		},
		{
			name:     "dot path",
			repo:     "my-app",
			branch:   "main",
			path:     ".",
			expected: "code/repos/my-app/main",
		},
		{
			name:     "custom branch",
			repo:     "my-app",
			branch:   "master",
			path:     "src/main.go",
			expected: "code/repos/my-app/master/src/main.go",
		},
		{
			name:     "path containing repo name prefix",
			repo:     "test",
			branch:   "main",
			path:     "test/readme.md",
			expected: "code/repos/test/main/readme.md",
		},
		{
			name:     "path being repo name",
			repo:     "test",
			branch:   "main",
			path:     "test",
			expected: "code/repos/test/main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RepoRelativeToWorkspace(tt.repo, tt.branch, tt.path)
			if got != tt.expected {
				t.Errorf("RepoRelativeToWorkspace(%q, %q, %q) = %q, want %q", tt.repo, tt.branch, tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsWorkspaceInternalPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"artifacts/diffs/cleanup.diff", true},
		{"logs/llm/call-001/prompt.md", true},
		{"specs/requirement.md", true},
		{"openspec/changes/readme/proposal.md", true},
		{"context/data.json", true},
		{"pr/description.md", true},
		{"code/repos/test/main/readme.md", false},
		{"readme.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsWorkspaceInternalPath(tt.path)
			if got != tt.expected {
				t.Errorf("IsWorkspaceInternalPath(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPathManagerRepoMainRelative(t *testing.T) {
	pm := NewPathManager("/tmp/workspaces")
	got := pm.RepoMainRelative("test", "main")
	want := "code/repos/test/main"
	if got != want {
		t.Errorf("RepoMainRelative(%q) = %q, want %q", "test", got, want)
	}
}

func TestPathManagerRepoWorktreeRelative(t *testing.T) {
	pm := NewPathManager("/tmp/workspaces")
	got := pm.RepoWorktreeRelative("test", "backend")
	want := "code/repos/test/worktrees/backend"
	if got != want {
		t.Errorf("RepoWorktreeRelative(%q, %q) = %q, want %q", "test", "backend", got, want)
	}
}
