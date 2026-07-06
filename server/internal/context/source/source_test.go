package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerAndCache(t *testing.T) {
	// 1. Setup Mock Workspace
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	
	err = os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	
	err = os.WriteFile(filepath.Join(tmpDir, "node_modules", "ignore.js"), []byte("ignore"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Test Scanner
	files, err := ScanRepository(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("Scanner failed to ignore directories. Expected 1 file, got %d", len(files))
	}

	if filepath.Base(files[0].Filepath) != "main.go" {
		t.Fatalf("Expected main.go, got %s", files[0].Filepath)
	}

	// 3. Test Cache
	cachePath := filepath.Join(tmpDir, "cache.db")
	cache, err := NewCache(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	tags := []Tag{
		{Name: "main", Kind: "def", Line: 1, Filepath: files[0].Filepath},
	}

	err = cache.SaveTags(files[0].Filepath, files[0].Mtime, tags)
	if err != nil {
		t.Fatal("Failed to save tags to cache:", err)
	}

	// 4. Test Cache Hit (Fresh)
	retrieved, fresh := cache.GetTagsIfFresh(files[0].Filepath, files[0].Mtime)
	if !fresh {
		t.Fatal("Expected fresh cache hit, got miss")
	}
	if len(retrieved) != 1 || retrieved[0].Name != "main" {
		t.Fatal("Cache data corruption")
	}

	// 5. Test Cache Miss (Stale)
	_, fresh = cache.GetTagsIfFresh(files[0].Filepath, files[0].Mtime+1)
	if fresh {
		t.Fatal("Expected cache miss due to mtime mismatch, got fresh")
	}
}

func TestScannerFiltering(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a binary file extension (should be ignored)
	err := os.WriteFile(filepath.Join(tmpDir, "image.png"), []byte{1, 2, 3}, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Create a source file (should be scanned)
	err = os.WriteFile(filepath.Join(tmpDir, "helper.go"), []byte("package main"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Create a large source file > 1MB (should be ignored)
	largeData := make([]byte, 1024*1024+10)
	err = os.WriteFile(filepath.Join(tmpDir, "large.go"), largeData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	files, err := ScanRepository(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected only helper.go to be scanned, got %d files", len(files))
	}

	if filepath.Base(files[0].Filepath) != "helper.go" {
		t.Fatalf("Expected helper.go, got %s", files[0].Filepath)
	}
}
