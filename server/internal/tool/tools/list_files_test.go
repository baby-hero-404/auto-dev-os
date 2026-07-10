package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestListFilesTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "list-test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test folder tree
	// tmpDir/
	//   file1.txt
	//   src/
	//     main.go
	//     utils/
	//       helper.go
	//   vendor/
	//     ignored.txt
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("txt"), 0o644)
	srcDir := filepath.Join(tmpDir, "src")
	os.Mkdir(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("go"), 0o644)
	utilsDir := filepath.Join(srcDir, "utils")
	os.Mkdir(utilsDir, 0o755)
	os.WriteFile(filepath.Join(utilsDir, "helper.go"), []byte("go helper"), 0o644)

	vendorDir := filepath.Join(tmpDir, "vendor")
	os.Mkdir(vendorDir, 0o755)
	os.WriteFile(filepath.Join(vendorDir, "ignored.txt"), []byte("ignored"), 0o644)

	lft := &ListFilesTool{}

	// Test 1: list files at max_depth=2
	res, err := lft.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"max_depth": 2,
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatalf("failed to execute list_files: %v", err)
	}
	if !res.Success {
		t.Fatalf("list_files failed: %s", res.Message)
	}

	output := res.Output
	// Expected lines:
	// .
	// ├── file1.txt
	// └── src/
	//     └── utils/
	// (utils/ is at depth 2, helper.go is at depth 3 and should NOT be listed)
	if !strings.Contains(output, "file1.txt") {
		t.Errorf("expected file1.txt in output, got:\n%s", output)
	}
	if !strings.Contains(output, "src/") {
		t.Errorf("expected src/ in output, got:\n%s", output)
	}
	if !strings.Contains(output, "utils/") {
		t.Errorf("expected utils/ in output, got:\n%s", output)
	}
	if strings.Contains(output, "helper.go") {
		t.Errorf("expected helper.go to be excluded at depth 2, got:\n%s", output)
	}
	if strings.Contains(output, "vendor") {
		t.Errorf("expected vendor to be excluded, got:\n%s", output)
	}
}
