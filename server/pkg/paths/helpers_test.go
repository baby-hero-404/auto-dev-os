package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-safe-path")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	root := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(root, 0755)

	goodFile := filepath.Join(root, "good.txt")
	_ = os.WriteFile(goodFile, []byte("hello"), 0644)

	evilFile := filepath.Join(tmpDir, "task-evil.txt")
	_ = os.WriteFile(evilFile, []byte("secret"), 0644)

	symlinkPath := filepath.Join(root, "bad_link.txt")
	_ = os.Symlink(evilFile, symlinkPath)

	tests := []struct {
		subPath string
		wantErr bool
	}{
		{"good.txt", false},
		{"./good.txt", false},
		{"../task/good.txt", false},
		{"../task-evil.txt", true},
		{"bad_link.txt", true},
		{"../task-evil/somefile", true},
	}

	for _, tc := range tests {
		_, err := ResolveSafePath(root, tc.subPath)
		if (err != nil) != tc.wantErr {
			t.Errorf("ResolveSafePath(%q, %q) error = %v, wantErr = %v", root, tc.subPath, err, tc.wantErr)
		}
	}
}
