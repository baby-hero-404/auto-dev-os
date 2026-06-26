package retrieval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

const (
	defaultSnippetLimit = 8
	maxFileBytes        = 256 * 1024
	maxSnippetLines     = 80
)

var (
	pathPattern       = regexp.MustCompile(`[A-Za-z0-9_./-]+\.[A-Za-z0-9]+`)
	wordPattern       = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_/-]{2,}`)
	symbolLinePattern = regexp.MustCompile(`^\s*(func|type|const|var|class|interface|export|function|async function)\b`)
)

type ContextRetriever interface {
	RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error)
}

type ContextKey string

const WorkspaceRootKey ContextKey = "retriever_workspace_root"

// SimpleFileRetriever retrieves relevant code snippets from a local checkout.
// It uses lexical-semantic scoring over paths, identifiers, and nearby content so
// it works without an external vector index or embedding service.
type SimpleFileRetriever struct {
	Root string
}

type scoredSnippet struct {
	models.ContextSnippet
	score float64
}

func NewSimpleFileRetriever(root string) *SimpleFileRetriever {
	return &SimpleFileRetriever{Root: root}
}

func (r *SimpleFileRetriever) RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error) {
	rootPath := r.Root
	if wsRoot, ok := ctx.Value(WorkspaceRootKey).(string); ok && wsRoot != "" {
		rootPath = wsRoot
	}
	if strings.TrimSpace(rootPath) == "" {
		return nil, fmt.Errorf("retriever root is required")
	}
	if limit <= 0 {
		limit = defaultSnippetLimit
	}

	root, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve retriever root: %w", err)
	}

	query := buildQuery(taskQuery)
	if len(query.terms) == 0 && len(query.paths) == 0 {
		return nil, nil
	}

	candidates := make([]scoredSnippet, 0, limit*4)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if shouldSkipDir(entry.Name(), rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCodeFile(rel) {
			return nil
		}

		info, err := entry.Info()
		if err != nil || info.Size() > maxFileBytes {
			return nil
		}

		snippets, err := r.snippetsForFile(root, rel, query)
		if err != nil {
			return nil
		}
		candidates = append(candidates, snippets...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			if candidates[i].Path == candidates[j].Path {
				return candidates[i].StartLine < candidates[j].StartLine
			}
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]models.ContextSnippet, 0, len(candidates))
	for _, candidate := range candidates {
		snippet := candidate.ContextSnippet
		snippet.Relevance = candidate.score
		results = append(results, snippet)
	}
	return results, nil
}

func (r *SimpleFileRetriever) snippetsForFile(root, rel string, query searchQuery) ([]scoredSnippet, error) {
	lines, err := readLines(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || len(lines) == 0 {
		return nil, err
	}

	fileScore := scoreText(rel, query, 2.5)
	if fileScore == 0 && !query.hasExplicitPath(rel) && !containsAnyTerm(lines, query) {
		return nil, nil
	}

	windows := snippetWindows(lines)
	scored := make([]scoredSnippet, 0, min(2, len(windows)))
	for _, window := range windows {
		content := strings.Join(lines[window.start:window.end], "\n")
		score := fileScore + scoreText(content, query, 1)
		if query.hasExplicitPath(rel) {
			score += 8
		}
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredSnippet{
			ContextSnippet: models.ContextSnippet{
				Source:    "filesystem",
				Path:      rel,
				StartLine: window.start + 1,
				EndLine:   window.end,
				Content:   content,
				Retriever: "semantic_file",
			},
			score: score,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	if len(scored) > 2 {
		scored = scored[:2]
	}
	return scored, nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), maxFileBytes)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

type lineWindow struct {
	start int
	end   int
}

func snippetWindows(lines []string) []lineWindow {
	if len(lines) <= maxSnippetLines {
		return []lineWindow{{start: 0, end: len(lines)}}
	}

	windows := []lineWindow{}
	for i, line := range lines {
		if !symbolLinePattern.MatchString(line) {
			continue
		}
		start := max(0, i-8)
		end := min(len(lines), start+maxSnippetLines)
		windows = append(windows, lineWindow{start: start, end: end})
	}
	if len(windows) == 0 {
		for start := 0; start < len(lines); start += maxSnippetLines {
			windows = append(windows, lineWindow{start: start, end: min(len(lines), start+maxSnippetLines)})
		}
	}
	return windows
}

type searchQuery struct {
	terms map[string]float64
	paths map[string]bool
}

func buildQuery(input string) searchQuery {
	query := searchQuery{
		terms: map[string]float64{},
		paths: map[string]bool{},
	}
	for _, match := range pathPattern.FindAllString(input, -1) {
		clean := filepath.ToSlash(filepath.Clean(match))
		if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
			continue
		}
		query.paths[clean] = true
		for _, term := range tokenize(clean) {
			query.terms[term] += 1.5
		}
	}
	for _, match := range wordPattern.FindAllString(input, -1) {
		for _, term := range tokenize(match) {
			if isStopWord(term) {
				continue
			}
			query.terms[term]++
		}
	}
	return query
}

func tokenize(value string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		token := strings.ToLower(current.String())
		if len(token) >= 3 {
			tokens = append(tokens, token)
		}
		current.Reset()
	}

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current.WriteRune(unicode.ToLower(r))
		case r == '_' || r == '-' || r == '/' || r == '.':
			flush()
		case unicode.IsUpper(r):
			flush()
			current.WriteRune(unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return tokens
}

func scoreText(value string, query searchQuery, weight float64) float64 {
	if len(query.terms) == 0 {
		return 0
	}
	text := strings.ToLower(value)
	score := 0.0
	for term, termWeight := range query.terms {
		if strings.Contains(text, term) {
			score += termWeight * weight
		}
	}
	return score
}

func containsAnyTerm(lines []string, query searchQuery) bool {
	for _, line := range lines {
		if scoreText(line, query, 1) > 0 {
			return true
		}
	}
	return false
}

func (q searchQuery) hasExplicitPath(rel string) bool {
	if q.paths[rel] {
		return true
	}
	for path := range q.paths {
		if strings.HasSuffix(rel, path) || strings.HasSuffix(path, rel) {
			return true
		}
	}
	return false
}

func shouldSkipDir(name, rel string) bool {
	switch name {
	case ".git", ".next", ".turbo", ".cache", "node_modules", "vendor", "dist", "build", "coverage", "tmp":
		return true
	}
	relClean := filepath.ToSlash(filepath.Clean(rel))
	if relClean == "logs" || strings.HasPrefix(relClean, "logs/") ||
		relClean == "artifacts" || strings.HasPrefix(relClean, "artifacts/") ||
		relClean == "openspec" || strings.HasPrefix(relClean, "openspec/") {
		return true
	}
	return strings.HasPrefix(relClean, "server/tmp/") || strings.HasPrefix(relClean, "web/.next/")
}

func isCodeFile(path string) bool {
	base := filepath.Base(path)
	if base == "patch.diff" || base == "task.json" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".rej" || ext == ".orig" {
		return false
	}
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".sql", ".md", ".json", ".yaml", ".yml", ".css":
		return true
	default:
		return false
	}
}

func isStopWord(term string) bool {
	switch term {
	case "the", "and", "for", "with", "this", "that", "from", "into", "task", "title", "description", "implement", "create", "update", "fix", "need", "needs":
		return true
	default:
		return false
	}
}
