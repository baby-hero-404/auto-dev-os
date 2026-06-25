package codecontext

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// ContextFile represents a source file included in the prompt context.
type ContextFile struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// ContextRetriever fetches relevant source code context for prompt assembly.
type ContextRetriever interface {
	Retrieve(ctx context.Context, task models.Task, repoPath string) ([]ContextFile, error)
}

// FileContextRetriever is the Phase 3b 4-tier priority stub.
type FileContextRetriever struct {
	Budget ContextBudget
}

// NewFileContextRetriever creates a retriever with default budget limits.
func NewFileContextRetriever() *FileContextRetriever {
	return &FileContextRetriever{Budget: DefaultBudget()}
}

func (r *FileContextRetriever) Retrieve(ctx context.Context, task models.Task, repoPath string) ([]ContextFile, error) {
	var paths []string

	if task.Analysis != nil {
		paths = append(paths, extractAffectedFiles(task.Analysis)...)
	}

	if len(paths) > 0 {
		neighbors := r.scanImportNeighbors(repoPath, paths)
		paths = appendUnique(paths, neighbors...)
	}

	if len(paths) == 0 {
		keywords := extractKeywords(task.Title + " " + task.Description)
		paths = r.searchByKeywords(repoPath, keywords)
	}

	paths = dedup(paths)
	if len(paths) > r.Budget.MaxFiles {
		paths = paths[:r.Budget.MaxFiles]
	}

	var files []ContextFile
	var totalBytes int
	for _, p := range paths {
		if totalBytes >= r.Budget.MaxTotalBytes {
			break
		}
		absPath := filepath.Join(repoPath, p)
		content, truncated, err := readFileBudget(absPath, r.Budget.MaxBytesPerFile)
		if err != nil {
			continue // Skip unreadable files silently.
		}
		totalBytes += len(content)
		files = append(files, ContextFile{
			Path:      p,
			Content:   content,
			Truncated: truncated,
		})
	}

	return files, nil
}

// goImportRe matches Go import statements: import "pkg/path" or import (..."pkg/path"...).
var goImportRe = regexp.MustCompile(`"([a-zA-Z0-9_./-]+)"`)

func (r *FileContextRetriever) scanImportNeighbors(repoPath string, files []string) []string {
	var neighbors []string
	for _, f := range files {
		absPath := filepath.Join(repoPath, f)
		imports := parseImports(absPath)
		for _, imp := range imports {
			// Only follow project-local imports (heuristic: contains /).
			if !strings.Contains(imp, "/") {
				continue
			}
			// Find last path segment and look for files.
			parts := strings.Split(imp, "/")
			// Try to find a matching directory in the repo.
			neighbors = append(neighbors, findDirFiles(repoPath, parts[len(parts)-1])...)
		}
	}
	return neighbors
}

func parseImports(filePath string) []string {
	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var imports []string
	scanner := bufio.NewScanner(f)
	inImportBlock := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}
		if inImportBlock || strings.HasPrefix(line, "import ") {
			matches := goImportRe.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) > 1 {
					imports = append(imports, m[1])
				}
			}
		}
	}
	return imports
}

func findDirFiles(repoPath, dirName string) []string {
	var found []string
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		// Skip common non-source directories.
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() && d.Name() == dirName {
			// Add all Go/TS/JS files in this directory.
			entries, _ := os.ReadDir(path)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				ext := filepath.Ext(e.Name())
				if ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
					rel, _ := filepath.Rel(repoPath, filepath.Join(path, e.Name()))
					found = append(found, rel)
				}
			}
			return filepath.SkipDir
		}
		return nil
	})
	return found
}
