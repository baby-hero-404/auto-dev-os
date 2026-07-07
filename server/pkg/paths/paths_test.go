package paths

import (
	"fmt"
	"sync"
	"testing"
)

func TestDirectory_Child_RootInvariant(t *testing.T) {
	root := NewDirectory("/workspace")

	// Standard nested child
	c1 := root.Child("tasks", "task-1")
	if c1.String() != "/workspace/tasks/task-1" {
		t.Errorf("expected /workspace/tasks/task-1, got %s", c1.String())
	}

	// Traversal attempt escaping root
	c2 := root.Child("../../etc/passwd")
	if c2.String() != "/workspace" {
		t.Errorf("expected traversal escape to be clamped to root /workspace, got %s", c2.String())
	}

	// File escape attempt
	f1 := root.File("../../etc/passwd")
	if f1.String() != "/workspace/passwd" {
		t.Errorf("expected file escape to be clamped to root directory file, got %s", f1.String())
	}
}

func TestInMemoryFileSystem(t *testing.T) {
	fs := NewInMemoryFileSystem()
	dir := NewDirectory("/data")
	file := dir.File("test.txt")

	if fs.Exists(file) {
		t.Error("expected file to not exist initially")
	}

	err := fs.WriteFile(file, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if !fs.Exists(file) {
		t.Error("expected file to exist after write")
	}

	if !fs.Exists(dir) {
		t.Error("expected directory to exist implicitly after child write")
	}

	data, err := fs.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(data))
	}
}

func TestConcurrentEnsureDir(t *testing.T) {
	fs := NewInMemoryFileSystem()
	dir := NewDirectory("/concurrency-test")

	var wg sync.WaitGroup
	workers := 20
	errChan := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subDir := dir.Child(fmt.Sprintf("worker-%d", id%5)) // multiple workers share dirs
			if err := fs.EnsureDir(subDir); err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("concurrent EnsureDir failed: %v", err)
	}
}
