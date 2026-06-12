package orchestrator

import (
	"bufio"
	"context"
	"fmt"
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
// Phase 3b implements a 4-tier priority stub; Phase 6 will swap in pgvector RAG.
type ContextRetriever interface {
	Retrieve(ctx context.Context, task models.Task, repoPath string) ([]ContextFile, error)
}

// ContextBudget defines hard limits to prevent context token explosion.
type ContextBudget struct {
	MaxFiles        int
	MaxBytesPerFile int
	MaxTotalBytes   int
}

// DefaultBudget returns sensible defaults for the 4-tier retriever.
func DefaultBudget() ContextBudget {
	return ContextBudget{
		MaxFiles:        8,
		MaxBytesPerFile: 20_000,
		MaxTotalBytes:   80_000,
	}
}

// FileContextRetriever is the Phase 3b 4-tier priority stub.
//
// Priority order:
//  1. Explicit files from task analysis (affected_files)
//  2. Import-neighbor scan — parse imports from explicit files
//  3. Keyword/path fallback — match task keywords against file paths
//  4. Token budget cap — truncate oversized files
type FileContextRetriever struct {
	Budget ContextBudget
}

// NewFileContextRetriever creates a retriever with default budget limits.
func NewFileContextRetriever() *FileContextRetriever {
	return &FileContextRetriever{Budget: DefaultBudget()}
}

func (r *FileContextRetriever) Retrieve(ctx context.Context, task models.Task, repoPath string) ([]ContextFile, error) {
	var paths []string

	// Tier 1: Explicit files from task analysis.
	if task.Analysis != nil {
		paths = append(paths, extractAffectedFiles(task.Analysis)...)
	}

	// Tier 2: Import-neighbor scan for explicit files.
	if len(paths) > 0 {
		neighbors := r.scanImportNeighbors(repoPath, paths)
		paths = appendUnique(paths, neighbors...)
	}

	// Tier 3: Keyword/path fallback if no explicit files.
	if len(paths) == 0 {
		keywords := extractKeywords(task.Title + " " + task.Description)
		paths = r.searchByKeywords(repoPath, keywords)
	}

	// Deduplicate and cap to budget.
	paths = dedup(paths)
	if len(paths) > r.Budget.MaxFiles {
		paths = paths[:r.Budget.MaxFiles]
	}

	// Tier 4: Read files with token budget enforcement.
	var (
		files      []ContextFile
		totalBytes int
	)
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

// ──────────────────────────────────────────────────────────────────
// Tier 1: Extract affected_files from task analysis JSON.
// ──────────────────────────────────────────────────────────────────

func extractAffectedFiles(raw []byte) []string {
	// Simple approach: scan for "affected_files" key in the JSON.
	// Avoids importing encoding/json to keep this lightweight.
	var files []string
	s := string(raw)
	idx := strings.Index(s, `"affected_files"`)
	if idx < 0 {
		return nil
	}
	// Find the array start.
	arrStart := strings.Index(s[idx:], "[")
	if arrStart < 0 {
		return nil
	}
	arrEnd := strings.Index(s[idx+arrStart:], "]")
	if arrEnd < 0 {
		return nil
	}
	arr := s[idx+arrStart : idx+arrStart+arrEnd+1]
	// Extract quoted strings from the array.
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindAllStringSubmatch(arr, -1)
	for _, m := range matches {
		if len(m) > 1 {
			files = append(files, m[1])
		}
	}
	return files
}

// ──────────────────────────────────────────────────────────────────
// Tier 2: Import-neighbor scan.
// ──────────────────────────────────────────────────────────────────

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
			for _, candidate := range findDirFiles(repoPath, parts[len(parts)-1]) {
				neighbors = append(neighbors, candidate)
			}
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

// ──────────────────────────────────────────────────────────────────
// Tier 3: Keyword/path fallback.
// ──────────────────────────────────────────────────────────────────

func extractKeywords(text string) []string {
	// Simple tokenization: lowercase, split on whitespace/punctuation.
	re := regexp.MustCompile(`[a-zA-Z_]{3,}`)
	tokens := re.FindAllString(strings.ToLower(text), -1)
	// Filter out very common stop words.
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"this": true, "that": true, "should": true, "will": true, "have": true,
		"not": true, "are": true, "was": true, "been": true, "also": true,
	}
	var keywords []string
	for _, t := range tokens {
		if !stop[t] {
			keywords = append(keywords, t)
		}
	}
	return keywords
}

func (r *FileContextRetriever) searchByKeywords(repoPath string, keywords []string) []string {
	var found []string
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == ".next") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".jsx" {
			return nil
		}

		rel, _ := filepath.Rel(repoPath, path)
		pathLower := strings.ToLower(rel)
		for _, kw := range keywords {
			if strings.Contains(pathLower, kw) {
				found = append(found, rel)
				break
			}
		}
		return nil
	})
	return found
}

// ──────────────────────────────────────────────────────────────────
// Tier 4: File reading with budget enforcement.
// ──────────────────────────────────────────────────────────────────

func readFileBudget(path string, maxBytes int) (string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("is a directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	if len(data) <= maxBytes {
		return string(data), false, nil
	}

	// Truncate and add marker.
	truncated := string(data[:maxBytes]) + "\n// ... truncated (file exceeds budget) ...\n"
	return truncated, true, nil
}

// ──────────────────────────────────────────────────────────────────
// Utility helpers.
// ──────────────────────────────────────────────────────────────────

func appendUnique(base []string, items ...string) []string {
	seen := make(map[string]bool, len(base))
	for _, b := range base {
		seen[b] = true
	}
	for _, item := range items {
		if !seen[item] {
			base = append(base, item)
			seen[item] = true
		}
	}
	return base
}

func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	return result
}
