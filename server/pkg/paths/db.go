package paths

import (
	"path/filepath"
)

// OSDatabasePaths implements DatabasePaths for the local OS filesystem.
type OSDatabasePaths struct {
	dataRoot string
}

// NewOSDatabasePaths creates a new OSDatabasePaths provider.
func NewOSDatabasePaths(dataRoot string) *OSDatabasePaths {
	return &OSDatabasePaths{dataRoot: filepath.ToSlash(filepath.Clean(dataRoot))}
}

// CacheDB returns the file path for the SQLite context engine cache database.
func (db *OSDatabasePaths) CacheDB() File {
	return NewDirectory(db.dataRoot).File("cache.db")
}
