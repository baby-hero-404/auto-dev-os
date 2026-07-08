package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanDirectory(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "scan-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Level 1 files/dirs
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Level 2 files/dirs (inside dir1)
	if err := os.WriteFile(filepath.Join(tmpDir, "dir1", "file2.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "dir1", "dir2"), 0755); err != nil {
		t.Fatal(err)
	}

	// Level 3 files/dirs (inside dir1/dir2)
	if err := os.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "file3.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "dir1", "dir2", "dir3"), 0755); err != nil {
		t.Fatal(err)
	}

	// Level 4 (should be skipped since depth limit is 3)
	if err := os.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "dir3", "file4.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Test 1: Full depth limit=3 scan
	result, err := ScanDirectory(tmpDir, 3, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that:
	// - file1.txt, dir1/ exist
	// - .git/ is skipped
	// - file2.txt, dir2/ exist
	// - file3.txt, dir3/ exist
	// - file4.txt is skipped (as it's depth 4)
	if strings.Contains(result, ".git") {
		t.Error("expected .git to be excluded")
	}
	if !strings.Contains(result, "file1.txt") {
		t.Error("expected file1.txt to be present")
	}
	if !strings.Contains(result, "dir1/") {
		t.Error("expected dir1/ to be present")
	}
	if !strings.Contains(result, "  file2.txt") {
		t.Error("expected file2.txt to be present at depth 2")
	}
	if !strings.Contains(result, "  dir2/") {
		t.Error("expected dir2/ to be present at depth 2")
	}
	if !strings.Contains(result, "    file3.txt") {
		t.Error("expected file3.txt to be present at depth 3")
	}
	if !strings.Contains(result, "    dir3/") {
		t.Error("expected dir3/ to be present at depth 3")
	}
	if strings.Contains(result, "file4.txt") {
		t.Error("expected file4.txt to be excluded due to depth limit")
	}

	// Test 2: Entry limit truncation
	truncatedResult, err := ScanDirectory(tmpDir, 3, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(truncatedResult), "\n")
	// The last line should start with "... and "
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "... and ") {
		t.Errorf("expected truncation message, got: %s", lastLine)
	}
}
