package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// LearnedSkillReader looks up active learned skills matching a query, for
// context_load to inject into a task's context (REQ-002).
type LearnedSkillReader interface {
	SearchActiveByText(ctx context.Context, projectID, query string, limit int) ([]models.LearnedSkill, error)
}

// ContextLoadStep implements Step for context loading.
type ContextLoadStep struct {
	rt            StepRuntime
	workspaceRoot string
	tasks         TaskReader
	status        StatusUpdater
	wkspace       WorkspaceLoader
	sandbox       SandboxRunner
	ctxEngine     provider.ContextEngine
	artifacts     ArtifactSaver
	repos         RepositoryLister
	log           Logger
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string
	learnedSkills LearnedSkillReader
}

func NewContextLoadStep(
	rt StepRuntime,
	workspaceRoot string,
	tasks TaskReader,
	status StatusUpdater,
	wkspace WorkspaceLoader,
	sandbox SandboxRunner,
	ctxEngine provider.ContextEngine,
	artifacts ArtifactSaver,
	repos RepositoryLister,
	log Logger,
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string,
	learnedSkills LearnedSkillReader,
) *ContextLoadStep {
	return &ContextLoadStep{
		rt:            rt,
		workspaceRoot: workspaceRoot,
		tasks:         tasks,
		status:        status,
		wkspace:       wkspace,
		sandbox:       sandbox,
		ctxEngine:     ctxEngine,
		artifacts:     artifacts,
		repos:         repos,
		log:           log,
		containerPath: containerPath,
		learnedSkills: learnedSkills,
	}
}

func (s *ContextLoadStep) ID() string { return workflow.StepContextLoad }

func (s *ContextLoadStep) StatusOnResume(_ StepResult) string {
	return models.TaskStatusContextLoading
}

func (s *ContextLoadStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	// Set once, up front, so every ctxEngine call below (IndexWorkspace, RetrieveContext,
	// GetRepoMap) sees the task's workspace root. Provider.GetRepoMap treats a missing
	// WorkspaceRootKey as "scanning the global root" and short-circuits to an empty result
	// (its safety check against scanning outside a task's workspace) — RetrieveContext and
	// GetRepoMap used to be called with the original, unaugmented ctx (only a locally-scoped
	// indexCtx carried the key, and only for the IndexWorkspace call), so the repo map / semantic
	// snippets silently came back empty even though the underlying AST indexing pipeline worked
	// correctly, forcing the analyze step to fall back to reading files one at a time via tools.
	ctx = context.WithValue(ctx, provider.WorkspaceRootKey, sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID))

	if s.rt.Task.Status == models.TaskStatusTodo || s.rt.Task.Status == models.TaskStatusFailed || s.rt.Task.Status == "" {
		if s.status != nil {
			if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusContextLoading); err != nil {
				return nil, fmt.Errorf("update task status: %w", err)
			}
		}
	}

	repoPaths := s.resolveContextRepoPaths(ctx)
	result := s.gatherRepoContexts(ctx, repoPaths)

	// Pre-warm the SQLite AST cache.
	if s.ctxEngine != nil {
		// 1. Two-Tier Cache Initialization
		var repoCommits []provider.RepoCommitInfo
		if s.wkspace != nil {
			if ws, errWS := s.wkspace.LoadTaskWorkspace(ctx, s.rt.Task); errWS == nil && ws != nil {
				for _, rWS := range ws.Repos {
					if s.rt.Task.RepositoryID != nil && rWS.RepoID != *s.rt.Task.RepositoryID {
						continue
					}
					if rWS.Paths.Main == "" {
						continue
					}
					repoAbs := filepath.Join(ws.Root, rWS.Paths.Main)
					if _, errStat := os.Stat(repoAbs); errStat == nil {
						commitHash, errCommit := runGitCmd(repoAbs, "rev-parse", "HEAD")
						if errCommit == nil && commitHash != "" {
							repoCommits = append(repoCommits, provider.RepoCommitInfo{
								RepoName:   rWS.Name,
								RepoPath:   repoAbs,
								CommitHash: commitHash,
							})
						}
					}
				}

				// Check for existence of the global cache, build if missing (Lazy Fallback Cache Builder)
				for _, rc := range repoCommits {
					globalCacheDir := s.ctxEngine.GetGlobalCacheDir()
					globalCachePath := filepath.Join(globalCacheDir, fmt.Sprintf("global_cache_%s_%s.db", rc.RepoName, rc.CommitHash))
					if _, errStat := os.Stat(globalCachePath); os.IsNotExist(errStat) {
						if s.log != nil {
							s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("global cache miss for repo %s at commit %s, building synchronously...", rc.RepoName, rc.CommitHash))
						}
						if errBuild := s.ctxEngine.BuildGlobalCache(rc.RepoPath, rc.RepoName, rc.CommitHash); errBuild != nil {
							if s.log != nil {
								s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to build global cache for repo %s: %v", rc.RepoName, errBuild))
							}
						}
					}
				}

				// Copy/merge the global cache to the local workspace's context/workspace_cache.db
				if len(repoCommits) > 0 {
					if errInit := s.ctxEngine.InitLocalCache(ws.Root, repoCommits); errInit != nil {
						if s.log != nil {
							s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to initialize local workspace cache: %v", errInit))
						}
					}
				}
			}
		}

		// 2. Perform incremental workspace indexing
		if err := s.ctxEngine.IndexWorkspace(ctx); err != nil {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to index workspace: %v", err))
			}
		}
	}

	// 3. Pre-compute and populate ContextCache (REQ-M02)
	var cache models.ContextCache
	if s.ctxEngine != nil {
		taskQuery := s.rt.Task.Title + "\n" + s.rt.Task.Description
		snippets, err := s.ctxEngine.RetrieveContext(ctx, taskQuery, 10)
		if err == nil {
			var modelsSnippets []models.ContextSnippet
			for _, sn := range snippets {
				modelsSnippets = append(modelsSnippets, models.ContextSnippet{
					Source:    sn.Source,
					Path:      sn.Path,
					StartLine: sn.StartLine,
					EndLine:   sn.EndLine,
					Content:   sn.Content,
					Relevance: sn.Relevance,
					Retriever: sn.Retriever,
				})
				cache.ActiveFiles = append(cache.ActiveFiles, sn.Path)
			}
			cache.SemanticSnippets = modelsSnippets
		}

		repoMap, err := s.ctxEngine.GetRepoMap(ctx, cache.ActiveFiles, 2048)
		if err == nil {
			cache.RepoMap = repoMap
		}
	}

	// Pre-compute ScanDirectory tree
	var treeBuilder strings.Builder
	for _, root := range repoPaths {
		if tree, err := ScanDirectory(root.path, 3, 200); err == nil && tree != "" {
			if root.prefix != "" {
				treeBuilder.WriteString(fmt.Sprintf("=== Repository %s ===\n%s\n\n", root.prefix, tree))
			} else {
				treeBuilder.WriteString(tree + "\n\n")
			}
		}
	}
	cache.DirectoryTree = strings.TrimSpace(treeBuilder.String())

	cacheJSON, _ := json.Marshal(cache)
	result["context_cache"] = string(cacheJSON)

	if s.learnedSkills != nil {
		// ~2k-token budget (REQ-002), approximated at 4 chars/token since we
		// don't have a tokenizer handy in this step.
		const learnedSkillsCharBudget = 8000
		taskQuery := s.rt.Task.Title + "\n" + s.rt.Task.Description
		if skills, err := s.learnedSkills.SearchActiveByText(ctx, s.rt.Task.ProjectID, taskQuery, 3); err == nil && len(skills) > 0 {
			var sb strings.Builder
			sb.WriteString("## Learned skills (from past tasks in this project)\n")
			ids := make([]string, 0, len(skills))
			for _, sk := range skills {
				section := fmt.Sprintf("### %s\n%s\n\n", sk.Title, sk.Content)
				if sb.Len()+len(section) > learnedSkillsCharBudget {
					break
				}
				sb.WriteString(section)
				ids = append(ids, sk.ID)
			}
			result["learned_skills"] = sb.String()
			result["skills_loaded"] = ids
		} else if err != nil && s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("learned skills search failed: %v", err))
		}
	}

	if s.artifacts != nil {
		_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepContextLoad, "context", result)
	}
	return result, nil
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

type contextSourceRoot struct {
	path   string
	prefix string
}

func (s *ContextLoadStep) resolveContextRepoPaths(ctx context.Context) []contextSourceRoot {
	localPath := sandbox.WorkspacePath(s.workspaceRoot, s.rt.Task.ID)
	var roots []contextSourceRoot

	var ws *models.TaskWorkspace
	var errWS error
	if s.wkspace != nil {
		ws, errWS = s.wkspace.LoadTaskWorkspace(ctx, s.rt.Task)
	} else {
		errWS = fmt.Errorf("wkspace manager is nil")
	}

	if errWS == nil && ws != nil {
		targetCount := 0
		for _, rWS := range ws.Repos {
			if s.rt.Task.RepositoryID != nil && rWS.RepoID != *s.rt.Task.RepositoryID {
				continue
			}
			if rWS.Paths.Main != "" {
				targetCount++
			}
		}
		for _, rWS := range ws.Repos {
			if s.rt.Task.RepositoryID != nil && rWS.RepoID != *s.rt.Task.RepositoryID {
				continue
			}
			if rWS.Paths.Main == "" {
				continue
			}
			repoAbs := filepath.Join(ws.Root, rWS.Paths.Main)
			if _, errStat := os.Stat(repoAbs); errStat == nil {
				prefix := ""
				if s.rt.Task.RepositoryID == nil && targetCount > 1 {
					prefix = rWS.Name
				}
				roots = append(roots, contextSourceRoot{path: repoAbs, prefix: prefix})
			}
		}
	}

	if len(roots) == 0 {
		roots = append(roots, contextSourceRoot{path: localPath, prefix: ""})
		if s.rt.Task.RepositoryID == nil {
			if files, err := os.ReadDir(localPath); err == nil {
				for _, f := range files {
					if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
						subPath := filepath.Join(localPath, f.Name())
						if _, errGit := os.Stat(filepath.Join(subPath, ".git")); errGit == nil {
							roots = append(roots, contextSourceRoot{path: subPath, prefix: f.Name()})
						} else if _, errMod := os.Stat(filepath.Join(subPath, "go.mod")); errMod == nil {
							roots = append(roots, contextSourceRoot{path: subPath, prefix: f.Name()})
						} else if _, errPkg := os.Stat(filepath.Join(subPath, "package.json")); errPkg == nil {
							roots = append(roots, contextSourceRoot{path: subPath, prefix: f.Name()})
						}
					}
				}
			}
		}
	}
	return roots
}

func (s *ContextLoadStep) gatherRepoContexts(ctx context.Context, repoPaths []contextSourceRoot) map[string]any {
	result := map[string]any{}
	gitLogs := map[string]string{}
	currentBranches := map[string]string{}
	testCommands := []string{}
	ciConfigs := []string{}
	conventions := map[string]string{}
	architectures := map[string]string{}
	contributings := map[string]string{}

	for _, root := range repoPaths {
		rp := root.path
		label := root.prefix
		if label == "" {
			label = "root"
		}
		pathPrefix := root.prefix

		var containerPath string
		if s.containerPath != nil {
			containerPath = s.containerPath(s.rt.Task, rp, "")
		}

		if gitLog := s.getSandboxGitOutput(ctx, label, containerPath, "get_git_log", "log -5 --oneline"); gitLog != "" {
			gitLogs[label] = gitLog
		}

		if branch := s.getSandboxGitOutput(ctx, label, containerPath, "get_git_branch", "rev-parse --abbrev-ref HEAD"); branch != "" {
			currentBranches[label] = branch
		}

		testCommands = append(testCommands, detectTestCommands(rp, label)...)
		ciConfigs = append(ciConfigs, detectCIConfigs(rp, pathPrefix)...)
		loadRepoDocs(rp, pathPrefix, conventions, architectures, contributings)
	}

	result["git_logs"] = gitLogs
	result["current_branches"] = currentBranches
	result["test_commands"] = testCommands
	result["ci_configs"] = ciConfigs
	result["conventions"] = conventions
	result["architectures"] = architectures
	result["contributings"] = contributings

	// Transfer build/lint script markers
	if len(repoPaths) > 0 {
		rp := repoPaths[0].path
		if pJsonData, err := os.ReadFile(filepath.Join(rp, "package.json")); err == nil {
			var pMap map[string]any
			if err := json.Unmarshal(pJsonData, &pMap); err == nil {
				if scripts, ok := pMap["scripts"].(map[string]any); ok {
					if _, ok := scripts["lint"]; ok {
						result["has_lint_script"] = true
					}
					if _, ok := scripts["build"]; ok {
						result["has_build_script"] = true
					}
				}
			}
		}
	}

	return result
}

func (s *ContextLoadStep) getSandboxGitOutput(ctx context.Context, rel, containerPath, stepPrefix, gitCmd string) string {
	cmd := fmt.Sprintf("git -C %s %s", paths.QuoteShellArg(containerPath), gitCmd)
	if s.sandbox != nil {
		if diffOutput, err := s.sandbox.RunCommand(ctx, s.rt.Task, s.rt.Agent, stepPrefix+"_"+rel, cmd); err == nil {
			if stdout, ok := diffOutput["stdout"].(string); ok && stdout != "" {
				return strings.TrimSpace(stdout)
			}
		}
	}
	return ""
}

func detectTestCommands(rp, rel string) []string {
	var cmds []string
	if _, err := os.Stat(filepath.Join(rp, "Makefile")); err == nil {
		cmds = append(cmds, fmt.Sprintf("make test (in %s)", rel))
	}
	if pJsonData, err := os.ReadFile(filepath.Join(rp, "package.json")); err == nil {
		var pMap map[string]any
		if err := json.Unmarshal(pJsonData, &pMap); err == nil {
			if scripts, ok := pMap["scripts"].(map[string]any); ok {
				if _, ok := scripts["test"]; ok {
					cmds = append(cmds, fmt.Sprintf("npm test (in %s)", rel))
				}
			}
		}
	}
	if _, err := os.Stat(filepath.Join(rp, "go.mod")); err == nil {
		cmds = append(cmds, fmt.Sprintf("go test ./... (in %s)", rel))
	}
	if _, err := os.Stat(filepath.Join(rp, "requirements.txt")); err == nil {
		cmds = append(cmds, fmt.Sprintf("pytest (in %s)", rel))
	} else if _, err := os.Stat(filepath.Join(rp, "pyproject.toml")); err == nil {
		cmds = append(cmds, fmt.Sprintf("pytest (in %s)", rel))
	}
	if _, err := os.Stat(filepath.Join(rp, "pom.xml")); err == nil {
		cmds = append(cmds, fmt.Sprintf("mvn test (in %s)", rel))
	}
	return cmds
}

func detectCIConfigs(rp, rel string) []string {
	var configs []string
	if files, err := os.ReadDir(filepath.Join(rp, ".github", "workflows")); err == nil {
		for _, f := range files {
			if !f.IsDir() {
				configs = append(configs, filepath.Join(rel, ".github", "workflows", f.Name()))
			}
		}
	}
	if _, err := os.Stat(filepath.Join(rp, ".gitlab-ci.yml")); err == nil {
		configs = append(configs, filepath.Join(rel, ".gitlab-ci.yml"))
	}
	return configs
}

func loadRepoDocs(rp, rel string, conventions, architectures, contributings map[string]string) {
	conventionFiles := []string{".editorconfig", ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".golangci.yml"}
	for _, file := range conventionFiles {
		if data, err := paths.ReadLimitedFile(filepath.Join(rp, file), 10000); err == nil {
			conventions[filepath.Join(rel, file)] = data
		}
	}
	if data, err := paths.ReadLimitedFile(filepath.Join(rp, "ARCHITECTURE.md"), 10000); err == nil {
		architectures[rel] = data
	}
	if data, err := paths.ReadLimitedFile(filepath.Join(rp, "CONTRIBUTING.md"), 10000); err == nil {
		contributings[rel] = data
	}
}
