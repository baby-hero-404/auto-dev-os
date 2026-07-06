package source

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Cache manages the SQLite-based on-demand cache for AST tags.
type Cache struct {
	db *sql.DB
}

// NewCache initializes a new SQLite connection and ensures the schema exists.
func NewCache(dbPath string) (*Cache, error) {
	// Ensure the parent directory exists so SQLite can create/open the DB file
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS file_cache (
			filepath TEXT PRIMARY KEY,
			mtime INTEGER NOT NULL,
			tags_json TEXT NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}

	return &Cache{db: db}, nil
}

// GetTagsIfFresh returns the cached tags if the provided mtime exactly matches the stored mtime.
// Returns false for the second boolean if the cache misses or is stale.
func (c *Cache) GetTagsIfFresh(filepath string, mtime int64) ([]Tag, bool) {
	var storedMtime int64
	var tagsJSON string

	err := c.db.QueryRow(`SELECT mtime, tags_json FROM file_cache WHERE filepath = ?`, filepath).Scan(&storedMtime, &tagsJSON)
	if err != nil {
		return nil, false // Cache miss or error
	}

	if storedMtime != mtime {
		return nil, false // Cache stale
	}

	var tags []Tag
	err = json.Unmarshal([]byte(tagsJSON), &tags)
	if err != nil {
		return nil, false // Corrupted cache payload
	}

	return tags, true
}

// SaveTags serializes the AST tags and saves them to the SQLite database, keyed by filepath.
func (c *Cache) SaveTags(filepath string, mtime int64, tags []Tag) error {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`
		INSERT INTO file_cache (filepath, mtime, tags_json)
		VALUES (?, ?, ?)
		ON CONFLICT(filepath) DO UPDATE SET mtime=excluded.mtime, tags_json=excluded.tags_json
	`, filepath, mtime, string(tagsJSON))

	return err
}

// Close gracefully shuts down the SQLite connection.
func (c *Cache) Close() error {
	return c.db.Close()
}
