package paths

import (
	"os"
)

// FileSystem defines the interface for executing filesystem side-effects.
type FileSystem interface {
	Exists(p Path) bool
	EnsureDir(d Directory) error
	ReadFile(f File) ([]byte, error)
	WriteFile(f File, data []byte, perm os.FileMode) error
}

// OSFileSystem is a production FileSystem that delegates directly to the 'os' package.
type OSFileSystem struct{}

// NewOSFileSystem creates an OS-backed FileSystem.
func NewOSFileSystem() FileSystem {
	return OSFileSystem{}
}

// Exists checks if the given path exists on the real filesystem.
func (OSFileSystem) Exists(p Path) bool {
	_, err := os.Stat(p.String())
	return err == nil
}

// EnsureDir creates all directories in the path if they don't exist.
func (OSFileSystem) EnsureDir(d Directory) error {
	return os.MkdirAll(d.String(), 0755)
}

// ReadFile reads the content of the file.
func (OSFileSystem) ReadFile(f File) ([]byte, error) {
	return os.ReadFile(f.String())
}

// WriteFile writes data to the file with the specified permissions.
func (OSFileSystem) WriteFile(f File, data []byte, perm os.FileMode) error {
	return os.WriteFile(f.String(), data, perm)
}
