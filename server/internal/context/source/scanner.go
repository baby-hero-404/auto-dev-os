package source

import (
	"os"
	"path/filepath"
	"strings"
)

// FileMeta holds the metadata necessary to determine if a file needs to be parsed.
type FileMeta struct {
	Filepath string
	Mtime    int64
}

// Tag represents a definition or reference extracted from a source file.
type Tag struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"` // "def" or "ref"
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Filepath string `json:"filepath"`
}

// ignoredDirs is a basic list of directories to ignore during the initial walk.
var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".codex":       true,
}

// ignoredExtensions is a set of known binary/non-text extensions to ignore.
var ignoredExtensions = map[string]bool{
	// Images
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".ico":  true,
	".svg":  true,
	// Archives
	".zip":  true,
	".tar":  true,
	".gz":   true,
	".rar":  true,
	".7z":   true,
	// Documents / Media
	".pdf":  true,
	".mp4":  true,
	".mp3":  true,
	".wav":  true,
	// Executables / Binaries / DBs
	".exe":  true,
	".bin":  true,
	".db":   true,
	".sqlite": true,
	".dll":  true,
	".so":   true,
	".dylib": true,
	// Fonts / Maps
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
	".map":   true,
}

// ScanRepository walks the directory tree starting from rootDir and collects
// file metadata while skipping common non-source directories, binary files, and large files.
func ScanRepository(rootDir string) ([]FileMeta, error) {
	var files []FileMeta

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Cannot access the path
		}

		if info.IsDir() {
			if ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files based on extension or size (> 1MB)
		ext := filepath.Ext(path)
		if ignoredExtensions[strings.ToLower(ext)] || info.Size() > 1024*1024 {
			return nil
		}

		// Append the file metadata. We use UnixNano for high-precision cache validation.
		files = append(files, FileMeta{
			Filepath: path,
			Mtime:    info.ModTime().UnixNano(),
		})

		return nil
	})

	return files, err
}
