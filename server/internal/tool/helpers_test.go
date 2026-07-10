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
}
