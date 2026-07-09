package paths

import (
	"testing"
)

func TestAgentPathContext(t *testing.T) {
	root := "/workspace/code/repos/tool_zentao/worktrees/backend"
	vfs := NewAgentPathContext(root, false, "tool_zentao", "backend")

	t.Run("ToLogical - single repo", func(t *testing.T) {
		logical, err := vfs.ToLogical("/workspace/code/repos/tool_zentao/worktrees/backend/internal/main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logical != "internal/main.go" {
			t.Errorf("expected 'internal/main.go', got '%s'", logical)
		}
	})

	t.Run("ToLogical - outside root", func(t *testing.T) {
		_, err := vfs.ToLogical("/workspace/code/repos/other_repo/main.go")
		if err == nil {
			t.Error("expected error for path outside root, got nil")
		}
	})

	t.Run("ToPhysical - clean relative path", func(t *testing.T) {
		physical, err := vfs.ToPhysical("internal/main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/workspace/code/repos/tool_zentao/worktrees/backend/internal/main.go"
		if physical != expected {
			t.Errorf("expected '%s', got '%s'", expected, physical)
		}
	})

	t.Run("ToPhysical - strip a/ b/", func(t *testing.T) {
		physical, err := vfs.ToPhysical("b/internal/main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/workspace/code/repos/tool_zentao/worktrees/backend/internal/main.go"
		if physical != expected {
			t.Errorf("expected '%s', got '%s'", expected, physical)
		}
	})

	t.Run("ToPhysical - strip redundant repo prefix", func(t *testing.T) {
		physical, err := vfs.ToPhysical("tool_zentao/internal/main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/workspace/code/repos/tool_zentao/worktrees/backend/internal/main.go"
		if physical != expected {
			t.Errorf("expected '%s', got '%s'", expected, physical)
		}
	})

	t.Run("ToPhysical - path traversal protection", func(t *testing.T) {
		_, err := vfs.ToPhysical("../../../secrets.txt")
		if err == nil {
			t.Error("expected error for path traversal, got nil")
		}
	})

	t.Run("ToPhysical - absolute path traversal protection", func(t *testing.T) {
		_, err := vfs.ToPhysical("/etc/passwd")
		if err == nil {
			t.Error("expected error for absolute path traversal, got nil")
		}
	})

	t.Run("ToLogical & ToPhysical - multi repo with prefix", func(t *testing.T) {
		multiRoot := "/workspace/code/repos"
		vfsMulti := NewAgentPathContext(multiRoot, true, "", "backend")

		logical, err := vfsMulti.ToLogical("/workspace/code/repos/tool_zentao/worktrees/backend/internal/main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logical != "tool_zentao/internal/main.go" {
			t.Errorf("expected 'tool_zentao/internal/main.go', got '%s'", logical)
		}

		physical, err := vfsMulti.ToPhysical("tool_zentao/internal/main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/workspace/code/repos/tool_zentao/worktrees/backend/internal/main.go"
		if physical != expected {
			t.Errorf("expected '%s', got '%s'", expected, physical)
		}
	})
}
