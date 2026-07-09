package tester

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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
			repoRel := paths.WorkspaceToRepoRelative(file)
			repoName := ""
			relFile = repoRel
			if idx := strings.Index(repoRel, "/"); idx != -1 {
				repoName = repoRel[:idx]
				relFile = repoRel[idx+1:]
			}
			if repoName != "" {
				workspaceRoot := filepath.Dir(repoHostPath)
				taskDirName := filepath.Base(repoHostPath)
				wp := paths.NewOSWorkspacePaths(workspaceRoot)
				repoMainHostPath := wp.RepoMain(taskDirName, repoName).String()
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

		if kind == ProjectGo {
			r.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running 'go mod tidy' on host in %s to resolve dependencies", absModDir))
			cmdTidy := exec.Command("go", "mod", "tidy")
			cmdTidy.Dir = absModDir
			if tidyErr := cmdTidy.Run(); tidyErr != nil {
				r.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to run 'go mod tidy' on host: %v", tidyErr))
			}
		} else if kind == ProjectJS {
			pkgJson := filepath.Join(absModDir, "package.json")
			if _, statErr := os.Stat(pkgJson); statErr == nil {
				r.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running 'npm install' on host in %s to resolve packages", absModDir))
				cmdInstall := exec.Command("npm", "install", "--no-audit", "--no-fund")
				cmdInstall.Dir = absModDir
				if installErr := cmdInstall.Run(); installErr != nil {
					r.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to run 'npm install' on host: %v", installErr))
				}
			}
		} else if kind == ProjectPython {
			reqsTxt := filepath.Join(absModDir, "requirements.txt")
			if _, statErr := os.Stat(reqsTxt); statErr == nil {
				r.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running 'pip install -r requirements.txt' on host in %s to resolve packages", absModDir))
				cmdPip := exec.Command("pip", "install", "-r", "requirements.txt")
				cmdPip.Dir = absModDir
				if pipErr := cmdPip.Run(); pipErr != nil {
					r.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to run pip install on host: %v", pipErr))
				}
			}
		} else if kind == ProjectJava {
			pomXml := filepath.Join(absModDir, "pom.xml")
			buildGradle := filepath.Join(absModDir, "build.gradle")
			if _, statErr := os.Stat(pomXml); statErr == nil {
				r.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running 'mvn dependency:resolve' on host in %s to resolve packages", absModDir))
				cmdMvn := exec.Command("mvn", "dependency:resolve")
				cmdMvn.Dir = absModDir
				if mvnErr := cmdMvn.Run(); mvnErr != nil {
					r.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to run maven resolve on host: %v", mvnErr))
				}
			} else if _, statErr := os.Stat(buildGradle); statErr == nil {
				r.Log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running './gradlew compileJava' on host in %s to resolve packages", absModDir))
				cmdGradle := exec.Command("./gradlew", "compileJava")
				cmdGradle.Dir = absModDir
				if gradleErr := cmdGradle.Run(); gradleErr != nil {
					r.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to run gradlew on host: %v", gradleErr))
				}
			}
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
