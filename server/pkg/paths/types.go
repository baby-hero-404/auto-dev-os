package paths

import (
	"path/filepath"
	"strings"
)

// Path is the general interface for any path representation.
type Path interface {
	String() string
}

// Directory represents a validated, immutable directory path.
type Directory struct {
	path string
}

// NewDirectory creates a new Directory value object.
func NewDirectory(p string) Directory {
	return Directory{path: filepath.ToSlash(filepath.Clean(p))}
}

// String returns the normalized slash-separated string representation.
func (d Directory) String() string {
	return d.path
}

// Child resolves a sub-path securely. It blocks Directory Traversal attacks (root escape).
func (d Directory) Child(elem ...string) Directory {
	joined := filepath.Join(append([]string{d.path}, elem...)...)
	cleaned := filepath.ToSlash(filepath.Clean(joined))

	// Security: prevent directory traversal by validating root invariant prefix.
	if d.path != "." && d.path != "" {
		rel, err := filepath.Rel(d.path, cleaned)
		if err != nil || strings.HasPrefix(rel, "..") {
			// Clamped to root directory to prevent escaping
			return d
		}
	}
	return Directory{path: cleaned}
}

// File constructs a File value object inside this directory, preventing escapes.
func (d Directory) File(name string) File {
	joined := filepath.Join(d.path, name)
	cleaned := filepath.ToSlash(filepath.Clean(joined))

	// Security: prevent directory traversal by validating root invariant prefix.
	if d.path != "." && d.path != "" {
		rel, err := filepath.Rel(d.path, cleaned)
		if err != nil || strings.HasPrefix(rel, "..") {
			// Clamped to folder boundary (use base name only)
			return File{path: filepath.ToSlash(filepath.Clean(filepath.Join(d.path, filepath.Base(name))))}
		}
	}
	return File{path: cleaned}
}

// File represents a validated, immutable file path.
type File struct {
	path string
}

// NewFile creates a new File value object.
func NewFile(p string) File {
	return File{path: filepath.ToSlash(filepath.Clean(p))}
}

// String returns the normalized slash-separated string representation.
func (f File) String() string {
	return f.path
}
