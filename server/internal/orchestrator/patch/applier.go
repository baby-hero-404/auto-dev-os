package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

type Runner struct {
	WorkspaceRoot            string
	GetTaskRepoHostPath      func(ctx context.Context, task *models.Task) (string, error)
	HostWorktreePath         func(task *models.Task, repoPath string, worktreeSuffix string) string
	ContainerPathForHostPath func(task *models.Task, hostPath string, worktreeSuffix string) string
	RunSandboxStepInWorktree func(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string, worktreeSuffix string) (map[string]any, error)
	GetDiff                  func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) (string, error)
	GetWorkspaceDiff         func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, worktreeSuffix string) (string, error)
	GetPRDiff                func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error)
	SandboxGetChangedFiles   func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string) ([]string, error)
	ListRepositories         func(ctx context.Context, projectID string) ([]models.Repository, error)
	LoadTaskWorkspace        func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error)
	GetRoleFromSuffix        func(suffix string) string
	UpdateTaskAnalysis       func(ctx context.Context, taskID string, analysis json.RawMessage) error
	Log                      func(ctx context.Context, taskID string, level string, message string)
}

func (r *Runner) ApplyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	if patchText == "" {
		return nil
	}
	patchText = CleanJunkLines(patchText)

	var ws *models.TaskWorkspace
	var errWS error
	if r.LoadTaskWorkspace != nil {
		ws, errWS = r.LoadTaskWorkspace(ctx, task)
	}

	role := ""
	if r.GetRoleFromSuffix != nil {
		role = r.GetRoleFromSuffix(worktreeSuffix)
	} else {
		s := strings.TrimPrefix(worktreeSuffix, "-")
		s = strings.TrimSuffix(s, "-worktree")
		switch s {
		case "be", "backend":
			role = "backend"
		case "fe", "frontend":
			role = "frontend"
		case "fix":
			role = "fix"
		default:
			role = s
		}
	}

	// Split patch by repository using the new normalized split helper
	repoPatches := r.SplitPatchByRepoWithWorkspace(patchText, ws, role)

	// Validate paths via AgentPathContext if present
	var pathCtx *paths.AgentPathContext
	if actx, ok := ctx.Value(paths.AgentPathContextKey).(*paths.AgentPathContext); ok {
		pathCtx = actx
	}
	if pathCtx != nil {
		for repoName, repoPatchText := range repoPatches {
			lines := strings.Split(repoPatchText, "\n")
			for _, line := range lines {
				var file string
				if strings.HasPrefix(line, "--- ") {
					file = strings.TrimPrefix(line, "--- ")
					file = strings.TrimSpace(file)
					file = strings.TrimPrefix(file, "a/")
				} else if strings.HasPrefix(line, "+++ ") {
					file = strings.TrimPrefix(line, "+++ ")
					file = strings.TrimSpace(file)
					file = strings.TrimPrefix(file, "b/")
				}
				if file != "" && file != "/dev/null" {
					logicalFile := file
					if pathCtx.UseRepoPrefix && repoName != "" && !strings.HasPrefix(logicalFile, repoName+"/") {
						logicalFile = repoName + "/" + logicalFile
					}
					_, err := pathCtx.ToPhysical(logicalFile)
					if err != nil {
						return &PolicyViolationError{
							Severity:   SeverityCritical,
							ErrorMsg:   fmt.Sprintf("security boundary violation: %v", err),
							Reason:     "unauthorized_path",
							Violations: []string{logicalFile},
						}
					}
				}
			}
		}
	}

	isRestore := strings.HasSuffix(stepID, "_restore")
	if task.Analysis != nil && !isRestore {
		var analysis models.TaskAnalysis
		if err := json.Unmarshal(task.Analysis, &analysis); err == nil {
			if len(analysis.ExecutionBoundaries) == 0 && len(analysis.AffectedFiles) > 0 {
				// Convert AffectedFiles into temporary boundaries for backward compatibility
				moduleMap := make(map[string]bool)
				for _, af := range analysis.AffectedFiles {
					dir := filepath.Dir(af.File)
					if dir == "." || dir == "/" {
						dir = ""
					}
					if !moduleMap[dir] {
						analysis.ExecutionBoundaries = append(analysis.ExecutionBoundaries, models.ExecutionBoundary{
							Module:       filepath.Base(dir),
							Root:         dir,
							Capabilities: []string{"modify_existing", "create_test", "create_helper"},
						})
						moduleMap[dir] = true
					}
				}
			}

			var errs []*PolicyViolationError
			var expansions []models.ExpandedBoundary

			for repoName, repoPatchText := range repoPatches {
				lines := strings.Split(repoPatchText, "\n")
				var currentOldFile string
				for _, line := range lines {
					if strings.HasPrefix(line, "--- ") {
						currentOldFile = strings.TrimPrefix(line, "--- ")
						currentOldFile = strings.TrimSpace(currentOldFile)
						currentOldFile = strings.TrimPrefix(currentOldFile, "a/")
					} else if strings.HasPrefix(line, "+++ ") {
						file := strings.TrimPrefix(line, "+++ ")
						file = strings.TrimSpace(file)
						file = strings.TrimPrefix(file, "b/")
						if file != "/dev/null" && file != "" {
							// Construct the repo-relative path for validation
							var repoRelPath string
							if repoName != "" {
								repoRelPath = repoName + "/" + file
							} else {
								repoRelPath = file
							}

							dec := EvaluatePolicy(repoRelPath, currentOldFile, &analysis)
							switch dec.Severity {
							case SeverityCritical:
								return &PolicyViolationError{
									Severity:   SeverityCritical,
									ErrorMsg:   dec.Reason,
									Reason:     "critical_violation",
									Violations: []string{repoRelPath},
								}
							case SeverityError:
								var allowedRoots []string
								for _, eb := range analysis.ExecutionBoundaries {
									allowedRoots = append(allowedRoots, eb.Root)
								}
								errs = append(errs, &PolicyViolationError{
									Severity:    SeverityError,
									ErrorMsg:    dec.Reason,
									Reason:      "outside_module",
									AllowedRoot: strings.Join(allowedRoots, ", "),
									Violations:  []string{repoRelPath},
								})
							case SeverityWarning, SeverityInfo:
								expansions = append(expansions, models.ExpandedBoundary{
									File:       repoRelPath,
									Reason:     dec.Reason,
									Capability: dec.Capability,
									Risk:       dec.Risk,
								})
								if r.Log != nil {
									r.Log(ctx, task.ID, "info", fmt.Sprintf("[%s] %s", dec.Severity, dec.Reason))
								}
							}
						}
					}
				}
			}

			if len(errs) > 0 {
				var allViolations []string
				var allowedRoots []string
				for _, eb := range analysis.ExecutionBoundaries {
					allowedRoots = append(allowedRoots, eb.Root)
				}
				for _, e := range errs {
					allViolations = append(allViolations, e.Violations...)
				}
				return &PolicyViolationError{
					Severity:    SeverityError,
					ErrorMsg:    fmt.Sprintf("unauthorized file modifications: %v", allViolations),
					Reason:      "outside_module",
					AllowedRoot: strings.Join(allowedRoots, ", "),
					Violations:  allViolations,
				}
			}

			if len(expansions) > 0 {
				analysis.ExpandedBoundaries = append(analysis.ExpandedBoundaries, expansions...)
				if r.UpdateTaskAnalysis != nil {
					newAnalysisBytes, marshalErr := json.Marshal(analysis)
					if marshalErr == nil {
						_ = r.UpdateTaskAnalysis(ctx, task.ID, newAnalysisBytes)
						task.Analysis = newAnalysisBytes
					}
				}
			}
		}
	}

	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)

	if task.RepositoryID != nil {
		// Single repo
		repoHostPath, err := r.GetTaskRepoHostPath(ctx, task)
		if err != nil {
			return err
		}

		repoName := ""
		if r.LoadTaskWorkspace != nil {
			if ws, errWS := r.LoadTaskWorkspace(ctx, task); errWS == nil && ws != nil {
				for _, rWS := range ws.Repos {
					if rWS.RepoID == *task.RepositoryID {
						repoName = rWS.Name
						break
					}
				}
			}
		}
		if repoName == "" {
			repoName = filepath.Base(filepath.Dir(repoHostPath))
		}

		cleanedPatch := patchText
		if repoName != "" {
			if p, ok := repoPatches[repoName]; ok && p != "" {
				cleanedPatch = p
			} else if p, ok := repoPatches[""]; ok && p != "" {
				cleanedPatch = p
			}
		}
		targetPath := r.HostWorktreePath(task, repoHostPath, worktreeSuffix)
		patchText = cleanedPatch
		fullPath := filepath.Join(targetPath, "patch.diff")
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(patchText), 0o644); err != nil {
			return err
		}
		defer os.Remove(fullPath)

		containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")
		containerPatchPath := filepath.Join(containerTargetPath, "patch.diff")

		cmd := fmt.Sprintf(`
ERR_LOG="/tmp/patch_err.log"
if git -C %[2]s apply --check -R %[3]s >/dev/null 2>&1; then
	true
elif git -C %[2]s apply --recount --whitespace=nowarn %[3]s >"$ERR_LOG" 2>&1; then
	true
elif patch --batch --no-backup-if-mismatch -d %[2]s -p1 < %[3]s >"$ERR_LOG" 2>&1; then
	true
else
	cat "$ERR_LOG" >&2
	(exit 2)
fi
CODE=$?
rm -f "$ERR_LOG"
find %[2]s -type f \( -name '*.orig' -o -name '*.rej' \) -delete
exit $CODE`,
			containerTargetPath,
			paths.QuoteShellArg(containerTargetPath),
			paths.QuoteShellArg(containerPatchPath),
		)
		_, err = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_apply_patch", cmd, worktreeSuffix)
		if err != nil {
			revertCmd := fmt.Sprintf(`
ERR_LOG="/tmp/patch_err.log"
if git -C %[2]s apply --reverse --check -R %[3]s >/dev/null 2>&1; then
	true
elif git -C %[2]s apply --reverse --recount --whitespace=nowarn %[3]s >"$ERR_LOG" 2>&1; then
	true
elif patch --batch --no-backup-if-mismatch -R -d %[2]s -p1 < %[3]s >"$ERR_LOG" 2>&1; then
	true
else
	cat "$ERR_LOG" >&2
	(exit 2)
fi
CODE=$?
rm -f "$ERR_LOG"
find %[2]s -type f \( -name '*.orig' -o -name '*.rej' \) -delete
exit $CODE`,
				containerTargetPath,
				paths.QuoteShellArg(containerTargetPath),
				paths.QuoteShellArg(containerPatchPath),
			)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch", revertCmd, worktreeSuffix)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
			return fmt.Errorf("git apply patch: %w", err)
		}
		_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
		return nil
	}

	// Multi-repo: split patch by repository
	for repoName, repoPatchText := range repoPatches {
		repoHostPath := ""
		if errWS == nil && ws != nil {
			for _, rWS := range ws.Repos {
				if strings.EqualFold(rWS.Name, repoName) {
					repoHostPath = filepath.Join(ws.Root, rWS.Paths.Main)
					break
				}
			}
		}
		if repoHostPath == "" {
			// Fallback to ReposPrefix + repoName/main
			repoDir := paths.NewOSWorkspacePaths(r.WorkspaceRoot).RepoRoot(task.ID, repoName).String()
			repoHostPath = filepath.Join(repoDir, "main")
			// Double fallback to localPath/repoName
			if stat, err := os.Stat(repoHostPath); err != nil || !stat.IsDir() {
				repoHostPath = filepath.Join(localPath, repoName)
			}
		}
		if ws != nil && len(ws.Repos) == 1 {
			repoHostPath = filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
			if repoName == "" {
				repoName = ws.Repos[0].Name
			}
		}
		repoWorktreeHostPath := r.HostWorktreePath(task, repoHostPath, worktreeSuffix)

		fullPath := filepath.Join(repoWorktreeHostPath, "patch.diff")
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(repoPatchText), 0o644); err != nil {
			return err
		}
		defer os.Remove(fullPath)

		containerRepoWorktreePath := r.ContainerPathForHostPath(task, repoWorktreeHostPath, "")
		containerPatchPath := filepath.Join(containerRepoWorktreePath, "patch.diff")

		// Use -p1 because splitPatchByRepo strips the workspace/repo prefix.
		cmd := fmt.Sprintf(`
ERR_LOG="/tmp/patch_err.log"
if git -C %[2]s apply --check -R -p1 %[3]s >/dev/null 2>&1; then
	true
elif git -C %[2]s apply -p1 --recount --whitespace=nowarn %[3]s >"$ERR_LOG" 2>&1; then
	true
elif patch --batch --no-backup-if-mismatch -d %[2]s -p1 < %[3]s >"$ERR_LOG" 2>&1; then
	true
else
	cat "$ERR_LOG" >&2
	(exit 2)
fi
CODE=$?
rm -f "$ERR_LOG"
find %[2]s -type f \( -name '*.orig' -o -name '*.rej' \) -delete
exit $CODE`,
			containerRepoWorktreePath,
			paths.QuoteShellArg(containerRepoWorktreePath),
			paths.QuoteShellArg(containerPatchPath),
		)
		_, err := r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_apply_patch_"+repoName, cmd, worktreeSuffix)
		if err != nil {
			revertCmd := fmt.Sprintf(`
ERR_LOG="/tmp/patch_err.log"
if git -C %[2]s apply --reverse --check -R -p1 %[3]s >/dev/null 2>&1; then
	true
elif git -C %[2]s apply --reverse -p1 --recount --whitespace=nowarn %[3]s >"$ERR_LOG" 2>&1; then
	true
elif patch --batch --no-backup-if-mismatch -R -d %[2]s -p1 < %[3]s >"$ERR_LOG" 2>&1; then
	true
else
	cat "$ERR_LOG" >&2
	(exit 2)
fi
CODE=$?
rm -f "$ERR_LOG"
find %[2]s -type f \( -name '*.orig' -o -name '*.rej' \) -delete
exit $CODE`,
				containerRepoWorktreePath,
				paths.QuoteShellArg(containerRepoWorktreePath),
				paths.QuoteShellArg(containerPatchPath),
			)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch_"+repoName, revertCmd, worktreeSuffix)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
			return fmt.Errorf("git apply patch failed for repo %s: %w", repoName, err)
		}
		_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
	}
	return nil
}
