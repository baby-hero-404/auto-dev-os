package mcpcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/context/repomap"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// Handlers wires the 6 MVP MCP tools to the existing internal/context/*
// packages and run_lint. It runs inside the CLI sandbox container, with the
// task workspace bind-mounted at RootDir — the same directory the host-side
// context_load step already indexes into WorkspaceCacheDbPath.
type Handlers struct {
	RootDir              string
	WorkspaceCacheDbPath string
	// ContextDir is where MaterializeCLIContext's file set (skills, learned
	// skills, task rules) is mounted for this run — normally
	// <container-workdir>/.autocode/context, exposed to the CLI subprocess
	// via the AUTOCODE_CONTEXT_DIR env var. skill.search reads from here
	// instead of hitting Postgres directly, since this process runs inside
	// the sandbox and has no DB credentials.
	ContextDir string
	provider   *provider.Provider
	// pathCtx carries RootDir as an AgentPathContext so provider.GetRepoMap's
	// two-tier "global vs per-task-workspace" safety check (which otherwise
	// treats rootDir == p.rootDir as "nothing to scan yet") doesn't short-circuit
	// for this process's single, fixed-root use.
	pathCtx *paths.AgentPathContext
}

func NewHandlers(rootDir, workspaceCacheDbPath, contextDir string) (*Handlers, error) {
	p, err := provider.NewProvider(rootDir, workspaceCacheDbPath)
	if err != nil {
		return nil, fmt.Errorf("init context provider: %w", err)
	}
	return &Handlers{
		RootDir:              rootDir,
		WorkspaceCacheDbPath: workspaceCacheDbPath,
		ContextDir:           contextDir,
		provider:             p,
		pathCtx:              paths.NewAgentPathContext(rootDir, false, "", ""),
	}, nil
}

// withPathCtx returns ctx carrying h.pathCtx under paths.AgentPathContextKey.
func (h *Handlers) withPathCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, paths.AgentPathContextKey, h.pathCtx)
}

// RegisterAll adds every implemented tool to the server.
func (h *Handlers) RegisterAll(s *Server) {
	s.Register(Tool{
		Def: toolDefinition{
			Name:        "repo.search",
			Description: "Semantic search over the repository's indexed AST tags. Returns matching code snippets with file/line locations.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`),
		},
		Handler: h.repoSearch,
	})
	s.Register(Tool{
		Def: toolDefinition{
			Name:        "ast.query",
			Description: "Look up a symbol (function/class/type) by name and return its exact definition location(s), without full source bodies.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"symbol":{"type":"string"},"limit":{"type":"integer"}},"required":["symbol"]}`),
		},
		Handler: h.astQuery,
	})
	s.Register(Tool{
		Def: toolDefinition{
			Name:        "architecture.query",
			Description: "Return a token-budgeted repo map (skeleton view of files ranked by relevance) for orienting in the codebase.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"active_files":{"type":"array","items":{"type":"string"}},"max_tokens":{"type":"integer"}}}`),
		},
		Handler: h.architectureQuery,
	})
	s.Register(Tool{
		Def: toolDefinition{
			Name:        "quality.check",
			Description: "Run the project's linter (golangci-lint) against the workspace or a subdirectory and return diagnostics.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
		Handler: h.qualityCheck,
	})
	s.Register(Tool{
		Def: toolDefinition{
			Name:        "dependency.impact",
			Description: "Return which files a given file depends on (calls into) and which files depend on it (callers), via the AST-derived call graph.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"file":{"type":"string"}},"required":["file"]}`),
		},
		Handler: h.dependencyImpact,
	})
	s.Register(Tool{
		Def: toolDefinition{
			Name:        "skill.search",
			Description: "Search the platform/learned skills already materialized for this task (relevant/skills/*.md, relevant/learned_skills.md) for text matching the query.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
		Handler: h.skillSearch,
	})
}

type repoSearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (h *Handlers) repoSearch(ctx context.Context, raw json.RawMessage) (string, error) {
	var args repoSearchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if err := h.provider.IndexAll(ctx); err != nil {
		return "", fmt.Errorf("repo.search index failed: %w", err)
	}
	snippets, err := h.provider.RetrieveContext(ctx, args.Query, args.Limit)
	if err != nil {
		return "", fmt.Errorf("repo.search failed: %w", err)
	}
	if len(snippets) == 0 {
		return "no matches found", nil
	}
	var sb strings.Builder
	for _, sn := range snippets {
		fmt.Fprintf(&sb, "%s:%d-%d\n```\n%s\n```\n\n", sn.Path, sn.StartLine, sn.EndLine, sn.Content)
	}
	return sb.String(), nil
}

type astQueryArgs struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit"`
}

func (h *Handlers) astQuery(ctx context.Context, raw json.RawMessage) (string, error) {
	var args astQueryArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if err := h.provider.IndexAll(ctx); err != nil {
		return "", fmt.Errorf("ast.query index failed: %w", err)
	}
	// Exact name match first: RetrieveContext is a fuzzy, multi-term
	// relevance search meant for natural-language queries and would return
	// "similar" tags/paths rather than the definition actually named here.
	// Both paths resolve via the mtime-checked SQLite tag cache (Dynamic
	// Context Invalidation), so a modified file is re-parsed either way.
	snippets, err := h.provider.FindExactSymbol(ctx, args.Symbol, args.Limit)
	if err != nil {
		return "", fmt.Errorf("ast.query failed: %w", err)
	}
	if len(snippets) == 0 {
		snippets, err = h.provider.RetrieveContext(ctx, args.Symbol, args.Limit)
		if err != nil {
			return "", fmt.Errorf("ast.query failed: %w", err)
		}
	}
	if len(snippets) == 0 {
		return "no symbol matches found for " + args.Symbol, nil
	}
	var sb strings.Builder
	for _, sn := range snippets {
		fmt.Fprintf(&sb, "%s:%d-%d\n", sn.Path, sn.StartLine, sn.EndLine)
	}
	return sb.String(), nil
}

type architectureQueryArgs struct {
	ActiveFiles []string `json:"active_files"`
	MaxTokens   int      `json:"max_tokens"`
}

func (h *Handlers) architectureQuery(ctx context.Context, raw json.RawMessage) (string, error) {
	var args architectureQueryArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if args.MaxTokens <= 0 {
		args.MaxTokens = 2048
	}
	repoMap, err := h.provider.GetRepoMap(h.withPathCtx(ctx), args.ActiveFiles, args.MaxTokens)
	if err != nil {
		return "", fmt.Errorf("architecture.query failed: %w", err)
	}
	if repoMap == "" {
		return "repo map is empty (workspace may not be indexed yet)", nil
	}
	return repoMap, nil
}

type qualityCheckArgs struct {
	Path string `json:"path"`
}

// qualityCheck shells out to golangci-lint directly (unlike the API-native
// RunLintTool, this process already runs inside the sandbox container, so it
// has no need to go through the host-side sandbox.Runtime indirection).
func (h *Handlers) qualityCheck(ctx context.Context, raw json.RawMessage) (string, error) {
	var args qualityCheckArgs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &args)
	}
	dir := h.RootDir
	if args.Path != "" {
		dir = h.RootDir + "/" + strings.TrimPrefix(args.Path, "/")
	}
	cmd := exec.CommandContext(ctx, "golangci-lint", "run", "--out-format=json")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if len(out) == 0 {
		return "no output from golangci-lint (is it installed in this image?)", nil
	}
	return string(out), nil
}

type dependencyImpactArgs struct {
	File string `json:"file"`
}

// dependencyImpact scans the workspace, extracts AST tags, and builds the
// same call-graph internal/context/provider.GetRepoMap ranks with — but
// returns raw graph edges for one file rather than a ranked repo map.
func (h *Handlers) dependencyImpact(ctx context.Context, raw json.RawMessage) (string, error) {
	var args dependencyImpactArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.File == "" {
		return "", fmt.Errorf("file is required")
	}
	target := filepath.Join(h.RootDir, args.File)

	// Reuse the mtime-checked SQLite tag cache (same one GetRepoMap/IndexAll
	// populate) instead of re-scanning and re-parsing every file on disk on
	// every call — that used to make this handler O(N) over the whole repo
	// per invocation, risking a timeout on large repos.
	allTags, err := h.provider.GetAllTags(h.withPathCtx(ctx), h.RootDir)
	if err != nil {
		return "", fmt.Errorf("dependency.impact scan failed: %w", err)
	}

	graph := repomap.NewDependencyGraph()
	graph.BuildGraph(allTags)

	dependsOn, dependents := graph.Neighbors(target)
	if len(dependsOn) == 0 && len(dependents) == 0 {
		return fmt.Sprintf("%s has no recorded call-graph edges (file not indexed, or has no cross-file def/ref matches)", args.File), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", args.File)
	if len(dependsOn) > 0 {
		sb.WriteString("depends on:\n")
		for _, f := range dependsOn {
			fmt.Fprintf(&sb, "  - %s\n", relTo(h.RootDir, f))
		}
	}
	if len(dependents) > 0 {
		sb.WriteString("depended on by:\n")
		for _, f := range dependents {
			fmt.Fprintf(&sb, "  - %s\n", relTo(h.RootDir, f))
		}
	}
	return sb.String(), nil
}

func relTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

type skillSearchArgs struct {
	Query string `json:"query"`
}

// skillSearch greps the already-materialized skill files for this task
// (written by PromptAssembler.MaterializeCLIContext under .autocode/context/
// inside the container) rather than querying Postgres — this process runs
// inside the sandbox and has no DB credentials.
func (h *Handlers) skillSearch(ctx context.Context, raw json.RawMessage) (string, error) {
	var args skillSearchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if h.ContextDir == "" {
		return "no materialized skills for this task (ContextDir not configured)", nil
	}

	var matches []string
	terms := strings.Fields(strings.ToLower(args.Query))
	err := filepath.Walk(h.ContextDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		lower := strings.ToLower(string(content))
		for _, term := range terms {
			if term != "" && strings.Contains(lower, term) {
				matches = append(matches, fmt.Sprintf("%s\n%s", relTo(h.ContextDir, path), string(content)))
				break
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("skill.search failed: %w", err)
	}
	if len(matches) == 0 {
		return "no materialized skill matched the query", nil
	}
	return strings.Join(matches, "\n\n---\n\n"), nil
}
