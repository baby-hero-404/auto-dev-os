package paths

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing/fstest"
)

// InMemoryFileSystem is a testing implementation of FileSystem using an in-memory map.
type InMemoryFileSystem struct {
	mu    sync.RWMutex
	files fstest.MapFS
}

// NewInMemoryFileSystem creates a new empty in-memory filesystem.
func NewInMemoryFileSystem() *InMemoryFileSystem {
	return &InMemoryFileSystem{
		files: make(fstest.MapFS),
	}
}

// Exists checks if the given path exists in the memory filesystem.
func (im *InMemoryFileSystem) Exists(p Path) bool {
	im.mu.RLock()
	defer im.mu.RUnlock()

	cleaned := filepath.ToSlash(filepath.Clean(p.String()))
	if cleaned == "." || cleaned == "/" || cleaned == "" {
		return true
	}

	if _, ok := im.files[cleaned]; ok {
		return true
	}

	// Check if it's a directory (i.e. prefix of another file)
	prefix := cleaned + "/"
	for name := range im.files {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// EnsureDir simulates creating directories by placing a placeholder in the in-memory map.
func (im *InMemoryFileSystem) EnsureDir(d Directory) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	cleaned := filepath.ToSlash(filepath.Clean(d.String()))
	if cleaned != "." && cleaned != "/" && cleaned != "" {
		placeholder := filepath.ToSlash(filepath.Clean(filepath.Join(cleaned, ".keep")))
		im.files[placeholder] = &fstest.MapFile{
			Data: []byte(""),
		}
	}
	return nil
}

// ReadFile reads the content of the file from memory.
func (im *InMemoryFileSystem) ReadFile(f File) ([]byte, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	cleaned := filepath.ToSlash(filepath.Clean(f.String()))
	file, ok := im.files[cleaned]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return file.Data, nil
}

// WriteFile writes the content of the file to memory.
func (im *InMemoryFileSystem) WriteFile(f File, data []byte, perm os.FileMode) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	cleaned := filepath.ToSlash(filepath.Clean(f.String()))
	im.files[cleaned] = &fstest.MapFile{
		Data: data,
		Mode: perm,
	}
	return nil
}
