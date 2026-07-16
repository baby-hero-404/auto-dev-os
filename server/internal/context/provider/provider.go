package provider

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/repomap"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
	"github.com/auto-code-os/auto-code-os/server/internal/context/symbol"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type ContextKey string

const WorkspaceRootKey ContextKey = "retriever_workspace_root"

type RepoCommitInfo struct {
	RepoName   string
	RepoPath   string // absolute path to repository in workspace
	CommitHash string
}

// ContextEngine defines the API contract for the LLM Orchestrator.
type ContextEngine interface {
	GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error)
	RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error)
	IndexWorkspace(ctx context.Context) error
	Close() error
	GetGlobalCacheDir() string
	BuildGlobalCache(repoAbsPath string, repoName string, commitHash string) error
	InitLocalCache(wsRoot string, repoCommits []RepoCommitInfo) error
}

// Provider implements ContextEngine.
type Provider struct {
	rootDir              string
	workspaceCacheDbPath string
	cache                *source.Cache
}

// NewProvider creates a new contextual engine attached to a workspace.
func NewProvider(rootDir string, workspaceCacheDbPath string) (*Provider, error) {
	cache, err := source.NewCache(workspaceCacheDbPath)
	if err != nil {
		return nil, err
	}
	return &Provider{
		rootDir:              rootDir,
		workspaceCacheDbPath: workspaceCacheDbPath,
		cache:                cache,
	}, nil
}

// Close releases the SQLite cache lock.
func (p *Provider) Close() error {
	return p.cache.Close()
}

func (p *Provider) getCache(ctx context.Context) (*source.Cache, func(), error) {
	wsRoot, ok := ctx.Value(WorkspaceRootKey).(string)
	if !ok || wsRoot == "" || wsRoot == p.rootDir {
		return p.cache, func() {}, nil
	}
	dbPath := filepath.Join(wsRoot, "context", "workspace_cache.db")
	c, err := source.NewCache(dbPath)
	if err != nil {
		return nil, nil, err
	}
	return c, func() { c.Close() }, nil
}

func runGitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

func mergeSqliteDBs(targetPath, sourcePath string) error {
	db, err := sql.Open("sqlite3", targetPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("ATTACH DATABASE ? AS source_db; INSERT OR IGNORE INTO file_cache SELECT * FROM source_db.file_cache; DETACH DATABASE source_db;", sourcePath)
	return err
}

func (p *Provider) buildGlobalCache(repoAbsPath string, repoName string, commitHash string, globalCachePath string) error {
	tmpPath := globalCachePath + ".tmp"
	_ = os.Remove(tmpPath)

	cache, err := source.NewCache(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		cache.Close()
		_ = os.Remove(tmpPath)
	}()

	filesMeta, err := source.ScanRepository(repoAbsPath)
	if err != nil {
		return err
	}

	for _, f := range filesMeta {
		extractedTags, err := symbol.ExtractTags(f.Filepath)
		if err == nil {
			_ = cache.SaveTags(f.Filepath, f.Mtime, extractedTags)
		}
	}

	cache.Close()

	if err := os.Rename(tmpPath, globalCachePath); err != nil {
		return err
	}

	// GC: Keep only this latest version, delete older versions for this repository
	globalCacheDir := filepath.Dir(globalCachePath)
	entries, err := os.ReadDir(globalCacheDir)
	if err == nil {
		prefix := fmt.Sprintf("global_cache_%s_", repoName)
		expectedName := filepath.Base(globalCachePath)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && entry.Name() != expectedName {
				_ = os.Remove(filepath.Join(globalCacheDir, entry.Name()))
			}
		}
	}

	return nil
}

func (p *Provider) initLocalCacheFromGlobal(wsRoot string, localDbPath string) error {
	metaJSONPath := filepath.Join(wsRoot, "metadata.json")
	metaData, err := os.ReadFile(metaJSONPath)
	if err != nil {
		_, err := source.NewCache(localDbPath)
		return err
	}

	var metadata models.TaskWorkspaceMetadata
	if err := json.Unmarshal(metaData, &metadata); err != nil {
		_, err := source.NewCache(localDbPath)
		return err
	}

	globalCacheDir := p.GetGlobalCacheDir()
	if err := os.MkdirAll(globalCacheDir, 0755); err != nil {
		_, err := source.NewCache(localDbPath)
		return err
	}

	var copiedFirst bool
	for _, r := range metadata.Repos {
		if r.Paths.Main == "" {
			continue
		}
		repoAbsPath := filepath.Join(wsRoot, r.Paths.Main)
		commitHash, err := runGitCmd(repoAbsPath, "rev-parse", "HEAD")
		if err != nil {
			continue
		}

		globalCachePath := filepath.Join(globalCacheDir, fmt.Sprintf("global_cache_%s_%s.db", r.Name, commitHash))

		if _, errStat := os.Stat(globalCachePath); os.IsNotExist(errStat) {
			_ = p.buildGlobalCache(repoAbsPath, r.Name, commitHash, globalCachePath)
		}

		if _, errStat := os.Stat(globalCachePath); errStat == nil {
			if !copiedFirst {
				if errCopy := copyFile(globalCachePath, localDbPath); errCopy == nil {
					copiedFirst = true
				}
			} else {
				_ = mergeSqliteDBs(localDbPath, globalCachePath)
			}
		}
	}

	if !copiedFirst {
		c, err := source.NewCache(localDbPath)
		if err != nil {
			return err
		}
		c.Close()
	}

	return nil
}

// GetRepoMap orchestrates the complete context pipeline to return an LLM-friendly string.
func (p *Provider) GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error) {
	rootDir := p.rootDir
	if wsRoot, ok := ctx.Value(WorkspaceRootKey).(string); ok && wsRoot != "" {
		rootDir = wsRoot
	}

	var pathCtx *paths.AgentPathContext
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok {
		pathCtx = actx
	}

	// Option B: Safety Check to prevent scanning the global workspace directory.
	// If the requested rootDir is the same as the configured global rootDir, skip scanning.
	if rootDir == p.rootDir && pathCtx == nil {
		return "", nil
	}

	// Only scan under 'code/repos' if it exists to isolate repository files from task metadata.
	scanDir := rootDir
	if pathCtx != nil {
		scanDir = pathCtx.PhysicalRoot()
	} else {
		wp := paths.NewOSWorkspacePaths(filepath.Dir(rootDir))
		reposDir := wp.CodeRoot(filepath.Base(rootDir)).String()
		if stat, err := os.Stat(reposDir); err == nil && stat.IsDir() {
			scanDir = reposDir
		}
	}

	// 1. Source Discovery
	filesMeta, err := source.ScanRepository(scanDir)
	if err != nil {
		return "", err
	}

	cache, cleanup, err := p.getCache(ctx)
	if err != nil {
		return "", err
	}
	defer cleanup()

	// 2. Extract or Load Tags via mtime SQLite Cache
	var allTags []source.Tag
	for _, f := range filesMeta {
		tags, fresh := cache.GetTagsIfFresh(f.Filepath, f.Mtime)
		if !fresh {
			extractedTags, err := symbol.ExtractTags(f.Filepath)
			if err == nil {
				tags = extractedTags
				_ = cache.SaveTags(f.Filepath, f.Mtime, tags)
			}
		}

		// Rewrite filepath to be relative to the task rootDir so that the returned repo map
		// references files clean and relative (e.g. 'code/repos/my_repo/main/main.go')
		var relPath string
		var relErr error
		if pathCtx != nil {
			relPath, relErr = pathCtx.ToLogical(f.Filepath)
		} else {
			relPath, relErr = filepath.Rel(rootDir, f.Filepath)
			if relErr == nil {
				relPath = filepath.ToSlash(relPath)
			}
		}

		if relErr == nil {
			for i := range tags {
				tags[i].Filepath = relPath
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

	localContextDir := filepath.Join(rootDir, "context")
	if err := os.MkdirAll(localContextDir, 0755); err != nil {
		return err
	}

	localDbPath := filepath.Join(localContextDir, "workspace_cache.db")
	if _, err := os.Stat(localDbPath); os.IsNotExist(err) {
		if errInit := p.initLocalCacheFromGlobal(rootDir, localDbPath); errInit != nil {
			emptyCache, errEmpty := source.NewCache(localDbPath)
			if errEmpty == nil {
				emptyCache.Close()
			}
		}
	}

	cache, err := source.NewCache(localDbPath)
	if err != nil {
		return err
	}
	defer cache.Close()

	scanDir := rootDir
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok && actx != nil {
		scanDir = actx.PhysicalRoot()
	} else {
		wp := paths.NewOSWorkspacePaths(filepath.Dir(rootDir))
		reposDir := wp.CodeRoot(filepath.Base(rootDir)).String()
		if stat, err := os.Stat(reposDir); err == nil && stat.IsDir() {
			scanDir = reposDir
		}
	}

	filesMeta, err := source.ScanRepository(scanDir)
	if err != nil {
		return err
	}

	for _, f := range filesMeta {
		_, fresh := cache.GetTagsIfFresh(f.Filepath, f.Mtime)
		if !fresh {
			extractedTags, err := symbol.ExtractTags(f.Filepath)
			if err == nil {
				_ = cache.SaveTags(f.Filepath, f.Mtime, extractedTags)
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

	cache, cleanup, err := p.getCache(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tags, err := cache.SearchTags(taskQuery, limit)
	if err != nil {
		return nil, err
	}

	rootDir := p.rootDir
	if wsRoot, ok := ctx.Value(WorkspaceRootKey).(string); ok && wsRoot != "" {
		rootDir = wsRoot
	}

	var pathCtx *paths.AgentPathContext
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok {
		pathCtx = actx
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

		var relPath string
		var relErr error
		if pathCtx != nil {
			relPath, relErr = pathCtx.ToLogical(t.Filepath)
		} else {
			relPath, relErr = filepath.Rel(rootDir, t.Filepath)
			if relErr == nil {
				relPath = filepath.ToSlash(relPath)
			}
		}

		if relErr != nil || relPath == "" || strings.HasPrefix(relPath, "..") {
			continue
		}

		snippets = append(snippets, models.ContextSnippet{
			Source:    "filesystem",
			Path:      relPath,
			StartLine: startLine,
			EndLine:   endLine,
			Content:   content,
			Relevance: t.Score,
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

func (p *Provider) GetGlobalCacheDir() string {
	if p.workspaceCacheDbPath == ":memory:" {
		dir := filepath.Join(p.rootDir, "..", ".data", "database", "global_cache")
		if _, err := os.Stat(filepath.Join(p.rootDir, "..", ".data")); err == nil {
			return dir
		}
		return "./.data/database/global_cache"
	}
	return filepath.Join(filepath.Dir(p.workspaceCacheDbPath), "global_cache")
}

func (p *Provider) BuildGlobalCache(repoAbsPath string, repoName string, commitHash string) error {
	globalCacheDir := p.GetGlobalCacheDir()
	if err := os.MkdirAll(globalCacheDir, 0755); err != nil {
		return err
	}
	globalCachePath := filepath.Join(globalCacheDir, fmt.Sprintf("global_cache_%s_%s.db", repoName, commitHash))
	return p.buildGlobalCache(repoAbsPath, repoName, commitHash, globalCachePath)
}

func (p *Provider) InitLocalCache(wsRoot string, repoCommits []RepoCommitInfo) error {
	localContextDir := filepath.Join(wsRoot, "context")
	if err := os.MkdirAll(localContextDir, 0755); err != nil {
		return err
	}
	localDbPath := filepath.Join(localContextDir, "workspace_cache.db")

	globalCacheDir := p.GetGlobalCacheDir()
	if err := os.MkdirAll(globalCacheDir, 0755); err != nil {
		return err
	}

	var copiedFirst bool
	for _, rc := range repoCommits {
		globalCachePath := filepath.Join(globalCacheDir, fmt.Sprintf("global_cache_%s_%s.db", rc.RepoName, rc.CommitHash))

		if _, errStat := os.Stat(globalCachePath); errStat == nil {
			if !copiedFirst {
				if errCopy := copyFile(globalCachePath, localDbPath); errCopy == nil {
					copiedFirst = true
				}
			} else {
				_ = mergeSqliteDBs(localDbPath, globalCachePath)
			}
		}
	}

	if !copiedFirst {
		c, err := source.NewCache(localDbPath)
		if err != nil {
			return err
		}
		c.Close()
	}

	return nil
}
