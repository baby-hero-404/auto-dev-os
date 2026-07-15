package tool

import (
	"path/filepath"
	"testing"
)

func TestSafeWorkspacePath(t *testing.T) {
	ws := "/tmp/workspace"

	// Valid path
	path, err := SafeWorkspacePath(ws, "src/main.go")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	expected := filepath.Join(ws, "src/main.go")
	if path != expected {
		t.Errorf("Expected %q, got %q", expected, path)
	}

	// Traversing upwards
	_, errTraverse := SafeWorkspacePath(ws, "../outside.go")
	if errTraverse == nil {
		t.Errorf("Expected traversal error")
	}

	// Traversing using absolute path
	_, errAbs := SafeWorkspacePath(ws, "/etc/passwd")
	if errAbs == nil {
		t.Errorf("Expected traversal error for absolute path")
	}

	// Trick path (prefix match but no directory boundary)
	// e.g. /tmp/workspace-other relative to /tmp/workspace
	_, errTrick := SafeWorkspacePath(ws, "../workspace-other")
	if errTrick == nil {
		t.Errorf("Expected traversal error for sibling directory trick")
	}

	t.Run("self-nesting guard inside repo checkout", func(t *testing.T) {
		repoWs := "/tmp/code/repos/tool_zentao/main"
		_, errGuard := SafeWorkspacePath(repoWs, "code/repos/tool_zentao/main/cmd/sync/main.go")
		if errGuard == nil {
			t.Fatalf("expected guard rejection error, got nil")
		}
		expectedErrStr := `path "code/repos/tool_zentao/main/cmd/sync/main.go" appears workspace-prefixed; this workspace is the repository root — use "cmd/sync/main.go"`
		if errGuard.Error() != expectedErrStr {
			t.Errorf("expected error %q, got %q", expectedErrStr, errGuard.Error())
		}
	})

	t.Run("self-nesting guard worktrees format", func(t *testing.T) {
		repoWs := "/tmp/code/repos/tool_zentao/worktrees/backend"
		_, errGuard := SafeWorkspacePath(repoWs, "code/repos/tool_zentao/worktrees/backend/cmd/sync/main.go")
		if errGuard == nil {
			t.Fatalf("expected guard rejection error, got nil")
		}
		expectedErrStr := `path "code/repos/tool_zentao/worktrees/backend/cmd/sync/main.go" appears workspace-prefixed; this workspace is the repository root — use "cmd/sync/main.go"`
		if errGuard.Error() != expectedErrStr {
			t.Errorf("expected error %q, got %q", expectedErrStr, errGuard.Error())
		}
	})

	t.Run("self-nesting guard does not trigger on legitimate code directory", func(t *testing.T) {
		repoWs := "/tmp/code/repos/tool_zentao/main"
		path, err := SafeWorkspacePath(repoWs, "code/foo.go")
		if err != nil {
			t.Fatalf("expected no error for legitimate code directory, got %v", err)
		}
		expected := filepath.Join(repoWs, "code/foo.go")
		if path != expected {
			t.Errorf("expected %q, got %q", expected, path)
		}
	})
}
