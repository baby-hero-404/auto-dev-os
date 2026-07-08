package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanDirectory walks the filesystem starting at root up to maxDepth (1-indexed).
// It ignores .git and node_modules directories, formats directories with a trailing slash,
// and represents hierarchy with 2-space indentation. Max maxEntries are returned.
func ScanDirectory(root string, maxDepth int, maxEntries int) (string, error) {
	var entries []string
	var count int
	var totalTruncated int

	var walk func(path string, depth int) error
	walk = func(path string, depth int) error {
		if depth > maxDepth {
			return nil
		}

		files, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, file := range files {
			name := file.Name()
			if name == ".git" || name == "node_modules" {
				continue
			}

			display := name
			if file.IsDir() {
				display += "/"
			}

			indent := strings.Repeat("  ", depth-1)
			
			if count >= maxEntries {
				totalTruncated++
				continue
			}

			entries = append(entries, fmt.Sprintf("%s%s", indent, display))
			count++

			if file.IsDir() {
				if err := walk(filepath.Join(path, name), depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(root, 1); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	result := strings.Join(entries, "\n")
	if totalTruncated > 0 {
		result += fmt.Sprintf("\n... and %d more files", totalTruncated)
	}

	return result, nil
}
