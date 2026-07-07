package provider

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/repomap"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
	"github.com/auto-code-os/auto-code-os/server/internal/context/symbol"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type ContextKey string

const WorkspaceRootKey ContextKey = "retriever_workspace_root"

// ContextEngine defines the API contract for the LLM Orchestrator.
type ContextEngine interface {
	GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error)
	RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error)
	IndexWorkspace(ctx context.Context) error
	Close() error
}

// Provider implements ContextEngine.
type Provider struct {
	rootDir string
	cache   *source.Cache
}

// NewProvider creates a new contextual engine attached to a workspace.
func NewProvider(rootDir string, cacheDbPath string) (*Provider, error) {
	cache, err := source.NewCache(cacheDbPath)
	if err != nil {
		return nil, err
	}
	return &Provider{
		rootDir: rootDir,
		cache:   cache,
	}, nil
}

// Close releases the SQLite cache lock.
func (p *Provider) Close() error {
	return p.cache.Close()
}

// GetRepoMap orchestrates the complete context pipeline to return an LLM-friendly string.
func (p *Provider) GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error) {
	rootDir := p.rootDir
	if wsRoot, ok := ctx.Value(WorkspaceRootKey).(string); ok && wsRoot != "" {
		rootDir = wsRoot
	}

	// Option B: Safety Check to prevent scanning the global workspace directory.
	// If the requested rootDir is the same as the configured global rootDir, skip scanning.
	if rootDir == p.rootDir {
		return "", nil
	}

	// Only scan under 'code/repos' if it exists to isolate repository files from task metadata.
	scanDir := rootDir
	reposDir := filepath.Join(rootDir, "code", "repos")
	if stat, err := os.Stat(reposDir); err == nil && stat.IsDir() {
		scanDir = reposDir
	}

	// 1. Source Discovery
	filesMeta, err := source.ScanRepository(scanDir)
	if err != nil {
		return "", err
	}

	// 2. Extract or Load Tags via mtime SQLite Cache
	var allTags []source.Tag
	for _, f := range filesMeta {
		tags, fresh := p.cache.GetTagsIfFresh(f.Filepath, f.Mtime)
		if !fresh {
			extractedTags, err := symbol.ExtractTags(f.Filepath)
			if err == nil {
				tags = extractedTags
				_ = p.cache.SaveTags(f.Filepath, f.Mtime, tags)
			}
		}

		// Rewrite filepath to be relative to the task rootDir so that the returned repo map
		// references files clean and relative (e.g. 'code/repos/my_repo/main/main.go')
		relPath, err := filepath.Rel(rootDir, f.Filepath)
		if err == nil {
			for i := range tags {
				tags[i].Filepath = filepath.ToSlash(relPath)
			}
		}

		allTags = append(allTags, tags...)
	}

	// 3. Mathematical Modeling
	graph := repomap.NewDependencyGraph()
	graph.BuildGraph(allTags)

	// 4. Personalized Importance Ranking
	pageRank := graph.CalculatePageRank(activeFiles)

	// 5. Binary Pruning & Code-Body Stripping
	result := repomap.PruneTags(allTags, pageRank, maxTokens, repomap.FormatSkeleton, repomap.CountTokens)

	return result, nil
}

// IndexWorkspace loads AST tags into SQLite.
func (p *Provider) IndexWorkspace(ctx context.Context) error {
	rootDir := p.rootDir
	if wsRoot, ok := ctx.Value(WorkspaceRootKey).(string); ok && wsRoot != "" {
		rootDir = wsRoot
	}
	if rootDir == p.rootDir {
		return nil
	}
	scanDir := rootDir
	reposDir := filepath.Join(rootDir, "code", "repos")
	if stat, err := os.Stat(reposDir); err == nil && stat.IsDir() {
		scanDir = reposDir
	}
	filesMeta, err := source.ScanRepository(scanDir)
	if err != nil {
		return err
	}
	for _, f := range filesMeta {
		_, fresh := p.cache.GetTagsIfFresh(f.Filepath, f.Mtime)
		if !fresh {
			extractedTags, err := symbol.ExtractTags(f.Filepath)
			if err == nil {
				_ = p.cache.SaveTags(f.Filepath, f.Mtime, extractedTags)
			}
		}
	}
	return nil
}

// RetrieveContext reads AST definitions matching the query and returns their source code bodies.
func (p *Provider) RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error) {
	if err := p.IndexWorkspace(ctx); err != nil {
		return nil, err
	}

	tags, err := p.cache.SearchTags(taskQuery, limit)
	if err != nil {
		return nil, err
	}

	rootDir := p.rootDir
	if wsRoot, ok := ctx.Value(WorkspaceRootKey).(string); ok && wsRoot != "" {
		rootDir = wsRoot
	}

	var snippets []models.ContextSnippet
	for _, t := range tags {
		lines, err := readLines(t.Filepath)
		if err != nil {
			continue
		}

		startLine := t.Line
		endLine := t.EndLine
		if endLine <= startLine {
			endLine = startLine + 80
		}
		if endLine > len(lines) {
			endLine = len(lines)
		}
		if startLine < 1 {
			startLine = 1
		}

		content := strings.Join(lines[startLine-1:endLine], "\n")
		relPath, _ := filepath.Rel(rootDir, t.Filepath)
		if relPath == "" {
			relPath = t.Filepath
		}
		relPath = filepath.ToSlash(relPath)

		snippets = append(snippets, models.ContextSnippet{
			Source:    "filesystem",
			Path:      relPath,
			StartLine: startLine,
			EndLine:   endLine,
			Content:   content,
			Retriever: "ast_context_engine",
		})
	}

	return snippets, nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 256*1024)
	scanner.Buffer(buf, 256*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

