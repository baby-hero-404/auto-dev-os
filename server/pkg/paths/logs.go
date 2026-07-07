package paths

import (
	"path/filepath"
)

// OSLogPaths implements LogPaths for the local OS logging files.
type OSLogPaths struct {
	logRoot string
}

// NewOSLogPaths creates a new OSLogPaths provider.
func NewOSLogPaths(logRoot string) *OSLogPaths {
	return &OSLogPaths{logRoot: filepath.ToSlash(filepath.Clean(logRoot))}
}

// Root returns the base directory for local logs.
func (l *OSLogPaths) Root() Directory {
	return NewDirectory(l.logRoot)
}

// LogFile returns the file path for a specific log file name.
func (l *OSLogPaths) LogFile(name string) File {
	return l.Root().File(name)
}
