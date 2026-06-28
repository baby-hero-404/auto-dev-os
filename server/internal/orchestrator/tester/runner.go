package tester

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type Runner struct {
	ResolveRepoHostPath      func(context.Context, *models.Task) (string, error)
	HostWorktreePath         func(*models.Task, string, string) string
	ContainerPathForHostPath func(*models.Task, string, string) string
	RunSandboxStepInWorktree func(context.Context, *models.Task, *models.Agent, string, string, string) (map[string]any, error)
	SaveArtifact             func(context.Context, string, string, string, string, any) error
	Log                      func(context.Context, string, *string, string, string)
}

func (r Runner) RunTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error) {
	if len(changedFiles) == 0 {
		return map[string]any{"status": "skipped", "reason": "no changed files"}, nil
	}
	if err := r.validate(); err != nil {
		return nil, err
	}

	repoHostPath, err := r.ResolveRepoHostPath(ctx, task)
	if err != nil {
		return nil, err
	}
	mountedHostPath := r.HostWorktreePath(task, repoHostPath, worktreeSuffix)

	type moduleGroup struct {
		kind            ProjectKind
		files           []string
		goPackages      map[string]bool
		mountedHostPath string
	}

	groups := make(map[string]*moduleGroup)

	for _, file := range changedFiles {
		var fileMountedHostPath string
		var relFile string

		if task.RepositoryID != nil {
			fileMountedHostPath = mountedHostPath
			relFile = file
		} else {
			repoName := ""
			relFile = file
			if strings.HasPrefix(file, "code/repos/") {
				parts := strings.SplitN(file[len("code/repos/"):], "/", 3)
				if len(parts) >= 3 {
					if parts[1] == "worktrees" {
						subparts := strings.SplitN(file[len("code/repos/"):], "/", 4)
						if len(subparts) == 4 && subparts[1] == "worktrees" {
							repoName = subparts[0]
							relFile = subparts[3]
						} else {
							repoName = parts[0]
							relFile = parts[2]
						}
					} else {
						repoName = parts[0]
						relFile = parts[2]
					}
				}
			} else {
				if idx := strings.Index(file, "/"); idx != -1 {
					repoName = file[:idx]
					relFile = file[idx+1:]
				}
			}
			if repoName != "" {
				repoDir := filepath.Join(repoHostPath, "code", "repos", repoName)
				mainDirName := "main"
				if entries, errEntries := os.ReadDir(repoDir); errEntries == nil {
					for _, entry := range entries {
						if entry.IsDir() && entry.Name() != "worktrees" && !strings.Contains(entry.Name(), "-") {
							mainDirName = entry.Name()
							break
						}
					}
				}
				repoMainHostPath := filepath.Join(repoDir, mainDirName)
				fileMountedHostPath = r.HostWorktreePath(task, repoMainHostPath, worktreeSuffix)
			} else {
				fileMountedHostPath = repoHostPath
			}
		}

		kind, markers := DetectProjectKindNear(fileMountedHostPath, relFile)

		modRelDir := ""
		if len(markers) > 0 {
			if dir, rf, found := FindModuleDir(fileMountedHostPath, relFile, markers); found {
				modRelDir = dir
				relFile = rf
			}
		}

		absModDir := filepath.Join(fileMountedHostPath, modRelDir)
		g, ok := groups[absModDir]
		if !ok {
			g = &moduleGroup{
				goPackages:      make(map[string]bool),
				mountedHostPath: fileMountedHostPath,
			}
			groups[absModDir] = g
		}
		if g.kind == ProjectUnknown {
			g.kind = kind
		}
		if kind == ProjectGo {
			dir := filepath.Dir(relFile)
			if dir == "." {
				g.goPackages["."] = true
			} else {
				g.goPackages["./"+dir] = true
			}
		} else if kind != ProjectUnknown {
			g.files = append(g.files, relFile)
		}
	}

	var testErrors []string
	var testResults []map[string]any

	for absModDir, g := range groups {
		containerModPath := r.ContainerPathForHostPath(task, absModDir, "")
		modRelDir := ""
		if rel, errRel := filepath.Rel(g.mountedHostPath, absModDir); errRel == nil {
			modRelDir = rel
		}

		kind := g.kind
		if kind == ProjectUnknown {
			kind = DetectProjectKind(absModDir)
		}
		cmd, ok := TargetedTestCommand(kind, containerModPath, g.files, g.goPackages)
		if !ok {
			continue
		}
		detectedType := string(kind)

		r.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running targeted tests for %s in %s: %s", detectedType, containerModPath, cmd))

		out, err := r.RunSandboxStepInWorktree(ctx, task, agent, stepName, cmd, worktreeSuffix)
		if err != nil {
			r.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests execution failed in %s: %v", containerModPath, err))
			_ = r.SaveArtifact(ctx, jobID, task.ID, stepName, "targeted_test", map[string]any{
				"status":  "failed",
				"error":   err.Error(),
				"command": cmd,
				"type":    detectedType,
				"module":  modRelDir,
			})
			testErrors = append(testErrors, fmt.Sprintf("module %s: %v", modRelDir, err))
		} else {
			stdout, _ := out["stdout"].(string)
			result := map[string]any{
				"status":  "passed",
				"stdout":  stdout,
				"command": cmd,
				"type":    detectedType,
				"module":  modRelDir,
			}
			_ = r.SaveArtifact(ctx, jobID, task.ID, stepName, "targeted_test", result)
			testResults = append(testResults, result)
		}
	}

	if len(testErrors) > 0 {
		return nil, fmt.Errorf("targeted tests failed: %s", strings.Join(testErrors, "; "))
	}

	if len(testResults) == 0 {
		return map[string]any{"status": "skipped", "reason": "no tests ran"}, nil
	}

	return map[string]any{
		"status": "passed",
		"info":   fmt.Sprintf("%d test suites passed", len(testResults)),
	}, nil
}

func (r Runner) validate() error {
	if r.ResolveRepoHostPath == nil {
		return fmt.Errorf("tester runner missing ResolveRepoHostPath")
	}
	if r.HostWorktreePath == nil {
		return fmt.Errorf("tester runner missing HostWorktreePath")
	}
	if r.ContainerPathForHostPath == nil {
		return fmt.Errorf("tester runner missing ContainerPathForHostPath")
	}
	if r.RunSandboxStepInWorktree == nil {
		return fmt.Errorf("tester runner missing RunSandboxStepInWorktree")
	}
	if r.SaveArtifact == nil {
		return fmt.Errorf("tester runner missing SaveArtifact")
	}
	if r.Log == nil {
		return fmt.Errorf("tester runner missing Log")
	}
	return nil
}

func FindModuleDir(targetPath string, relFilePath string, markers []string) (string, string, bool) {
	absStart := filepath.Join(targetPath, relFilePath)
	dir := filepath.Dir(absStart)

	for {
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker)
			if stat, err := os.Stat(markerPath); err == nil && !stat.IsDir() {
				relDir, errRel := filepath.Rel(targetPath, dir)
				if errRel == nil {
					relFile, errFile := filepath.Rel(dir, absStart)
					if errFile == nil {
						return relDir, relFile, true
					}
				}
			}
		}
		if dir == targetPath || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", "", false
}
