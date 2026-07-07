package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	orchestratorworkspace "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
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
	ctxEngine     provider.ContextEngine
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
	ctxEngine provider.ContextEngine,
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
		ctxEngine:     ctxEngine,
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

	// Pre-warm the SQLite AST cache.
	if s.ctxEngine != nil {
		if err := s.ctxEngine.IndexWorkspace(ctx); err != nil {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("failed to index workspace: %v", err))
			}
		}
	}

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
