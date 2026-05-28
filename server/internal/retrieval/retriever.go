package retrieval

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

type ContextRetriever interface {
	RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error)
}

type SimpleFileRetriever struct {
	Root string
}

func NewSimpleFileRetriever(root string) *SimpleFileRetriever {
	return &SimpleFileRetriever{Root: root}
}

func (r *SimpleFileRetriever) RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error) {
	if strings.TrimSpace(r.Root) == "" {
		return nil, fmt.Errorf("retriever root is required")
	}
	if limit <= 0 {
		limit = 5
	}

	paths := extractCandidatePaths(taskQuery)
	snippets := make([]models.ContextSnippet, 0, min(limit, len(paths)))
	for _, path := range paths {
		if len(snippets) >= limit {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		snippet, err := r.readSnippet(path)
		if err != nil {
			continue
		}
		snippets = append(snippets, snippet)
	}
	return snippets, nil
}

func (r *SimpleFileRetriever) readSnippet(path string) (models.ContextSnippet, error) {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return models.ContextSnippet{}, fmt.Errorf("path outside retriever root")
	}
	fullPath := filepath.Join(r.Root, clean)
	file, err := os.Open(fullPath)
	if err != nil {
		return models.ContextSnippet{}, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && len(lines) < 80 {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return models.ContextSnippet{}, err
	}

	return models.ContextSnippet{
		Source:    "filesystem",
		Path:      clean,
		StartLine: 1,
		EndLine:   len(lines),
		Content:   strings.Join(lines, "\n"),
		Relevance: 0.5,
		Retriever: "simple_file",
	}, nil
}

func extractCandidatePaths(query string) []string {
	re := regexp.MustCompile(`[A-Za-z0-9_./-]+\.[A-Za-z0-9]+`)
	matches := re.FindAllString(query, -1)
	seen := map[string]bool{}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		clean := filepath.Clean(match)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		paths = append(paths, clean)
	}
	return paths
}
