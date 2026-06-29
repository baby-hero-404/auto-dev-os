package steps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	orchestratorworkspace "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// ContextLoadStep implements Step for context loading.
type ContextLoadStep struct {
	rt            StepRuntime
	workspaceRoot string
	tasks         TaskReader
	status        StatusUpdater
	wkspace       WorkspaceLoader
	sandbox       SandboxRunner
	llm           LLMChatter
	artifacts     ArtifactSaver
	repos         RepositoryLister
	log           Logger
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string
}

func NewContextLoadStep(
	rt StepRuntime,
	workspaceRoot string,
	tasks TaskReader,
	status StatusUpdater,
	wkspace WorkspaceLoader,
	sandbox SandboxRunner,
	llm LLMChatter,
	artifacts ArtifactSaver,
	repos RepositoryLister,
	log Logger,
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string,
) *ContextLoadStep {
	return &ContextLoadStep{
		rt:            rt,
		workspaceRoot: workspaceRoot,
		tasks:         tasks,
		status:        status,
		wkspace:       wkspace,
		sandbox:       sandbox,
		llm:           llm,
		artifacts:     artifacts,
		repos:         repos,
		log:           log,
		containerPath: containerPath,
	}
}

func (s *ContextLoadStep) ID() string { return workflow.StepContextLoad }

func (s *ContextLoadStep) StatusOnResume(_ StepResult) string {
	return models.TaskStatusContextLoading
}

func (s *ContextLoadStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	if s.rt.Task.Status == models.TaskStatusTodo || s.rt.Task.Status == models.TaskStatusFailed || s.rt.Task.Status == "" {
		if s.status != nil {
			if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusContextLoading); err != nil {
				return nil, fmt.Errorf("update task status: %w", err)
			}
		}
	}

	repoPaths := s.resolveContextRepoPaths(ctx)
	result := s.gatherRepoContexts(ctx, repoPaths)

	if s.artifacts != nil {
		_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepContextLoad, "context", result)
	}
	return result, nil
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

	missingDoc := len(architectures) == 0 || len(contributings) == 0 || len(conventions) == 0
	if missingDoc && len(repoPaths) > 0 {
		s.ensureRepoProfile(ctx, repoPaths[0].path, architectures, conventions)
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
	cmd := fmt.Sprintf("git -C %s %s", orchestratorworkspace.QuoteShellArg(containerPath), gitCmd)
	if s.sandbox != nil {
		if diffOutput, err := s.sandbox.RunCommand(ctx, s.rt.Task, s.rt.Agent, stepPrefix+"_"+rel, cmd); err == nil {
			if stdout, ok := diffOutput["stdout"].(string); ok && stdout != "" {
				return strings.TrimSpace(stdout)
			}
		}
	}
	return ""
}

func (s *ContextLoadStep) ensureRepoProfile(ctx context.Context, rp string, architectures, conventions map[string]string) {
	var repoURL string
	if s.rt.Task.RepositoryID != nil && s.repos != nil {
		if repos, err := s.repos.ListByProjectID(ctx, s.rt.Task.ProjectID); err == nil {
			for _, r := range repos {
				if r.ID == *s.rt.Task.RepositoryID {
					repoURL = r.URL
					break
				}
			}
		}
	}

	var containerPath string
	if s.containerPath != nil {
		containerPath = s.containerPath(s.rt.Task, rp, "")
	}
	if repoURL == "" {
		repoURL = s.getSandboxGitOutput(ctx, "remote", containerPath, "get_git_remote_origin", "remote get-url origin")
	}
	if repoURL == "" {
		repoURL = filepath.Base(rp)
	}

	repoURL = sanitizeRepoURL(repoURL)
	repoHash := getRepoHash(repoURL)
	currentCommit := s.getSandboxGitOutput(ctx, "commit", containerPath, "get_git_commit", "rev-parse HEAD")

	cacheFile := filepath.Join(filepath.Dir(s.workspaceRoot), "repositories", repoHash, "profile.json")
	var profile RepoProfile
	cacheHit := false

	if data, err := os.ReadFile(cacheFile); err == nil {
		if err := json.Unmarshal(data, &profile); err == nil {
			if currentCommit == "" || profile.CommitHash == currentCommit {
				cacheHit = true
			}
		}
	}

	if !cacheHit && s.llm != nil {
		profile, cacheHit = s.generateRepoProfile(ctx, rp, repoURL, currentCommit, cacheFile)
	}

	if cacheHit {
		var archBuilder strings.Builder
		archBuilder.WriteString(fmt.Sprintf("# Architecture (Cached Profile)\n\n%s\n\n## Directory Structure\n", profile.Architecture.Summary))
		for p, desc := range profile.Architecture.DirectoryStructure {
			archBuilder.WriteString(fmt.Sprintf("* `%s`: %s\n", p, desc))
		}
		architectures["cached_profile"] = archBuilder.String()

		var convBuilder strings.Builder
		convBuilder.WriteString(fmt.Sprintf("# Coding Conventions (Cached Profile)\n\n* **Languages**: %s\n* **Naming**: %s\n* **Linter Rules**: %s\n",
			profile.Conventions.Language, profile.Conventions.Naming, profile.Conventions.LinterRules))
		conventions["cached_profile"] = convBuilder.String()
	}
}

func (s *ContextLoadStep) generateRepoProfile(ctx context.Context, rp, repoURL, currentCommit, cacheFile string) (RepoProfile, bool) {
	var profile RepoProfile
	treeStr := scanRepoDirectoryTree(rp, 3)
	var configs []string
	configFiles := []string{"go.mod", "package.json", "Cargo.toml", "requirements.txt"}
	for _, file := range configFiles {
		if data, err := orchestratorworkspace.ReadLimitedFile(filepath.Join(rp, file), 2000); err == nil {
			configs = append(configs, fmt.Sprintf("=== %s ===\n%s", file, data))
		}
	}
	configsStr := strings.Join(configs, "\n\n")

	systemPrompt := "You are an expert codebase profiling agent. You analyze directory structures and config files to generate structured JSON profiles."
	userPrompt := fmt.Sprintf(`Analyze this repository and generate a structured JSON profile describing its architecture and coding conventions.

Directory tree:
"""
%s
"""

Configuration files:
"""
%s
"""

You MUST return a JSON object matching this exact structure:
{
  "repo_url": %q,
  "generated_at": %q,
  "commit_hash": %q,
  "architecture": {
    "summary": "<Short explanation of the system design and architecture pattern, e.g., clean architecture, MVC, etc.>",
    "directory_structure": {
      "path/to/folder1": "Description of what folder1 contains and its responsibility"
    }
  },
  "conventions": {
    "language": "<Primary languages used>",
    "naming": "<Naming conventions detected or assumed for files/functions/types>",
    "linter_rules": "<Expected linting style or rules based on configs>"
  }
}

Return ONLY the raw JSON object. Do not include markdown code block formatting (such as triple backticks with json) or any conversational text.`, treeStr, configsStr, repoURL, time.Now().Format(time.RFC3339), currentCommit)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	routeCtx := llm.WithRouteOptions(ctx, llm.RouteOptions{
		Complexity: models.TaskComplexityEasy,
		OrgID:      s.rt.Agent.OrgID,
		ProjectID:  s.rt.Task.ProjectID,
		AgentID:    s.rt.Agent.ID,
		TaskID:     s.rt.Task.ID,
		RouteName:  "profiler",
	})

	resp, err := s.llm.Chat(routeCtx, messages)
	if err == nil && resp != nil {
		cleaned := cleanJSONResponse(resp.Content)
		if errUnmarshal := json.Unmarshal([]byte(cleaned), &profile); errUnmarshal == nil {
			if data, errMarshal := json.MarshalIndent(profile, "", "  "); errMarshal == nil {
				_ = os.MkdirAll(filepath.Dir(cacheFile), 0755)
				_ = os.WriteFile(cacheFile, data, 0644)
			}
			return profile, true
		}
	}
	return profile, false
}

type RepoProfile struct {
	RepoURL      string           `json:"repo_url"`
	GeneratedAt  string           `json:"generated_at"`
	CommitHash   string           `json:"commit_hash"`
	Architecture RepoArchitecture `json:"architecture"`
	Conventions  RepoConventions  `json:"conventions"`
}

type RepoArchitecture struct {
	Summary            string            `json:"summary"`
	DirectoryStructure map[string]string `json:"directory_structure"`
}

type RepoConventions struct {
	Language    string `json:"language"`
	Naming      string `json:"naming"`
	LinterRules string `json:"linter_rules"`
}

func sanitizeRepoURL(urlStr string) string {
	if strings.Contains(urlStr, "@") {
		if strings.HasPrefix(urlStr, "https://") {
			parts := strings.SplitN(strings.TrimPrefix(urlStr, "https://"), "@", 2)
			if len(parts) == 2 {
				return "https://" + parts[1]
			}
		}
		if strings.HasPrefix(urlStr, "http://") {
			parts := strings.SplitN(strings.TrimPrefix(urlStr, "http://"), "@", 2)
			if len(parts) == 2 {
				return "http://" + parts[1]
			}
		}
	}
	return urlStr
}

func normalizeRepoURL(urlStr string) string {
	urlStr = strings.TrimPrefix(urlStr, "https://")
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "git@")
	if idx := strings.Index(urlStr, "@"); idx != -1 {
		urlStr = urlStr[idx+1:]
	}
	urlStr = strings.TrimSuffix(urlStr, ".git")
	urlStr = strings.ReplaceAll(urlStr, ":", "/")
	return strings.ToLower(strings.TrimSpace(urlStr))
}

func getRepoHash(urlStr string) string {
	norm := normalizeRepoURL(urlStr)
	h := sha256.New()
	h.Write([]byte(norm))
	return hex.EncodeToString(h.Sum(nil))
}

func cleanJSONResponse(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func scanRepoDirectoryTree(root string, maxDepth int) string {
	var builder strings.Builder
	var walk func(path string, depth int)
	walk = func(path string, depth int) {
		if depth > maxDepth {
			return
		}
		files, err := os.ReadDir(path)
		if err != nil {
			return
		}
		indent := strings.Repeat("  ", depth)
		for _, f := range files {
			if strings.HasPrefix(f.Name(), ".") && f.Name() != ".github" {
				continue
			}
			if f.Name() == "node_modules" || f.Name() == "vendor" || f.Name() == "dist" || f.Name() == "build" {
				continue
			}
			builder.WriteString(fmt.Sprintf("%s- %s", indent, f.Name()))
			if f.IsDir() {
				builder.WriteString("/\n")
				walk(filepath.Join(path, f.Name()), depth+1)
			} else {
				builder.WriteString("\n")
			}
		}
	}
	walk(root, 0)
	return builder.String()
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
		if data, err := orchestratorworkspace.ReadLimitedFile(filepath.Join(rp, file), 10000); err == nil {
			conventions[filepath.Join(rel, file)] = data
		}
	}
	if data, err := orchestratorworkspace.ReadLimitedFile(filepath.Join(rp, "ARCHITECTURE.md"), 10000); err == nil {
		architectures[rel] = data
	}
	if data, err := orchestratorworkspace.ReadLimitedFile(filepath.Join(rp, "CONTRIBUTING.md"), 10000); err == nil {
		contributings[rel] = data
	}
}
