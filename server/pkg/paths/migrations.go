package paths

import (
	"path/filepath"
)

// OSMigrationSource implements MigrationSource for local migrations directory.
type OSMigrationSource struct {
	migrationsRoot string
}

// NewOSMigrationSource creates a new OSMigrationSource provider.
func NewOSMigrationSource(migrationsRoot string) *OSMigrationSource {
	if migrationsRoot == "" {
		migrationsRoot = "migration"
	}
	return &OSMigrationSource{migrationsRoot: filepath.ToSlash(filepath.Clean(migrationsRoot))}
}

// Root returns the root migrations directory.
func (m *OSMigrationSource) Root() Directory {
	return NewDirectory(m.migrationsRoot)
}
