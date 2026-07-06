package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
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

	// Scan lines of patch to extract modified files
	lines := strings.Split(patchText, "\n")
	var modifiedFiles []string
	isNewFile := make(map[string]bool)
	allowedNewFiles := make(map[string]bool)
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
				modifiedFiles = append(modifiedFiles, file)
				if currentOldFile == "/dev/null" || currentOldFile == "" {
					isNewFile[file] = true
				}
			}
		}
	}

	// Enforce affected files if specified
	if task.Analysis != nil {
		var analysis models.TaskAnalysis
		if err := json.Unmarshal(task.Analysis, &analysis); err == nil && len(analysis.AffectedFiles) > 0 {
			// Validate all files modified in the patch against the allowed pattern list
			for _, file := range modifiedFiles {
				isAllowed := false
				for _, pattern := range analysis.AffectedFiles {
					if MatchAffectedFile(pattern, file) {
						isAllowed = true
						break
					}
				}
				if !isAllowed {
					if isNewFile[file] && IsSafeNewFilePath(file) && IsUnderAffectedDir(file, analysis.AffectedFiles) {
						if r.Log != nil {
							r.Log(ctx, task.ID, "warn", fmt.Sprintf("allowing new file %q under affected directory; this should be reviewed", file))
						}
						allowedNewFiles[file] = true
						continue
					}
					return fmt.Errorf("security violation: patch attempts to modify file %q which is not in the approved affected_files spec %v", file, analysis.AffectedFiles)
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
			repoPatches := SplitPatchByRepo(patchText)
			if p, ok := repoPatches[repoName]; ok && p != "" {
				cleanedPatch = p
			} else if p, ok := repoPatches[""]; ok && p != "" {
				cleanedPatch = p
			}
		}
		patchText = CleanPatchPaths(cleanedPatch)
		targetPath := r.HostWorktreePath(task, repoHostPath, worktreeSuffix)
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
elif patch --batch --no-backup-if-mismatch -d %[2]s -p0 < %[3]s >"$ERR_LOG" 2>&1; then
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
			workspace.QuoteShellArg(containerTargetPath),
			workspace.QuoteShellArg(containerPatchPath),
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
elif patch --batch --no-backup-if-mismatch -R -d %[2]s -p0 < %[3]s >"$ERR_LOG" 2>&1; then
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
				workspace.QuoteShellArg(containerTargetPath),
				workspace.QuoteShellArg(containerPatchPath),
			)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch", revertCmd, worktreeSuffix)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm -f %s", workspace.QuoteShellArg(containerPatchPath)), worktreeSuffix)
			return fmt.Errorf("git apply patch: %w", err)
		}
		if len(allowedNewFiles) > 0 {
			if err := r.appendNewAffectedFiles(ctx, task, allowedNewFiles); err != nil && r.Log != nil {
				r.Log(ctx, task.ID, "warn", fmt.Sprintf("failed to persist new affected files: %v", err))
			}
		}
		_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm -f %s", workspace.QuoteShellArg(containerPatchPath)), worktreeSuffix)
		return nil
	}

	// Multi-repo: split patch by repository
	var ws *models.TaskWorkspace
	var errWS error
	if r.LoadTaskWorkspace != nil {
		ws, errWS = r.LoadTaskWorkspace(ctx, task)
	}
	repoPatches := SplitPatchByRepo(patchText)
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
			// Fallback to ReposPrefix + repoName/<defaultBranch>
			repoDir := filepath.Join(localPath, workspace.ReposDirName, repoName)
			mainDirName := "main"
			if entries, errEntries := os.ReadDir(repoDir); errEntries == nil {
				for _, entry := range entries {
					if entry.IsDir() && entry.Name() != "worktrees" && !strings.Contains(entry.Name(), "-") {
						mainDirName = entry.Name()
						break
					}
				}
			}
			repoHostPath = filepath.Join(repoDir, mainDirName)
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
elif patch --batch --no-backup-if-mismatch -d %[2]s -p0 < %[3]s >"$ERR_LOG" 2>&1; then
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
			workspace.QuoteShellArg(containerRepoWorktreePath),
			workspace.QuoteShellArg(containerPatchPath),
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
elif patch --batch --no-backup-if-mismatch -R -d %[2]s -p0 < %[3]s >"$ERR_LOG" 2>&1; then
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
				workspace.QuoteShellArg(containerRepoWorktreePath),
				workspace.QuoteShellArg(containerPatchPath),
			)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch_"+repoName, revertCmd, worktreeSuffix)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm -f %s", workspace.QuoteShellArg(containerPatchPath)), worktreeSuffix)
			return fmt.Errorf("git apply patch failed for repo %s: %w", repoName, err)
		}
		if len(allowedNewFiles) > 0 {
			if err := r.appendNewAffectedFiles(ctx, task, allowedNewFiles); err != nil && r.Log != nil {
				r.Log(ctx, task.ID, "warn", fmt.Sprintf("failed to persist new affected files: %v", err))
			}
		}
		_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm -f %s", workspace.QuoteShellArg(containerPatchPath)), worktreeSuffix)
	}
	return nil
}

func (r *Runner) appendNewAffectedFiles(ctx context.Context, task *models.Task, files map[string]bool) error {
	if len(files) == 0 {
		return nil
	}

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		if err := json.Unmarshal(task.Analysis, &analysis); err != nil {
			return err
		}
	}

	changed := false
	existing := make(map[string]bool, len(analysis.AffectedFiles))
	for _, file := range analysis.AffectedFiles {
		existing[file] = true
	}
	for file := range files {
		if !existing[file] {
			analysis.AffectedFiles = append(analysis.AffectedFiles, file)
			existing[file] = true
			changed = true
		}
	}
	if !changed {
		return nil
	}

	raw, err := json.Marshal(analysis)
	if err != nil {
		return err
	}
	task.Analysis = raw
	if r.UpdateTaskAnalysis != nil {
		if err := r.UpdateTaskAnalysis(ctx, task.ID, raw); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) CaptureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)
	targetPath := r.HostWorktreePath(task, localPath, worktreeSuffix)
	containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")

	if task.RepositoryID != nil {
		repoHostPath, err := r.GetTaskRepoHostPath(ctx, task)
		if err != nil {
			return "", err
		}
		targetPath = r.HostWorktreePath(task, repoHostPath, worktreeSuffix)
		containerTargetPath = r.ContainerPathForHostPath(task, targetPath, "")

		return r.GetDiff(ctx, task, agent, containerTargetPath)
	}

	// Multi-repo diff
	return r.GetWorkspaceDiff(ctx, task, agent, containerTargetPath, worktreeSuffix)
}

func (r *Runner) CapturePRDiff(ctx context.Context, task *models.Task, agent *models.Agent, baseBranch string) (string, error) {
	localPath := sandbox.WorkspacePath(r.WorkspaceRoot, task.ID)
	targetPath := localPath
	containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")

	var ws *models.TaskWorkspace
	var errWS error
	if r.LoadTaskWorkspace != nil {
		ws, errWS = r.LoadTaskWorkspace(ctx, task)
	}

	if task.RepositoryID != nil {
		repoHostPath, err := r.GetTaskRepoHostPath(ctx, task)
		if err != nil {
			return "", err
		}
		targetPath = r.HostWorktreePath(task, repoHostPath, "")
		containerTargetPath = r.ContainerPathForHostPath(task, targetPath, "")

		resolvedBase := baseBranch
		if errWS == nil && ws != nil {
			for _, rWS := range ws.Repos {
				if rWS.RepoID == *task.RepositoryID && rWS.DefaultBranch != "" {
					resolvedBase = rWS.DefaultBranch
					break
				}
			}
		}

		return r.GetPRDiff(ctx, task, agent, containerTargetPath, resolvedBase)
	}

	if r.ListRepositories == nil {
		return "", fmt.Errorf("multi-repo PR diff requires repository listing")
	}
	repos, err := r.ListRepositories(ctx, task.ProjectID)
	if err != nil {
		return "", err
	}

	var diffOut []string
	for _, repo := range repos {
		repoHostPath := ""
		resolvedBase := baseBranch
		if errWS == nil && ws != nil {
			for _, rWS := range ws.Repos {
				if rWS.RepoID == repo.ID {
					repoHostPath = filepath.Join(ws.Root, rWS.Paths.Main)
					if rWS.DefaultBranch != "" {
						resolvedBase = rWS.DefaultBranch
					}
					break
				}
			}
		}
		if repoHostPath == "" {
			repoName := repoNameFromURL(repo.URL)
			repoHostPath = filepath.Join(localPath, "code", "repos", repoName, "main")
			if stat, statErr := os.Stat(repoHostPath); statErr != nil || !stat.IsDir() {
				repoHostPath = filepath.Join(localPath, repoName)
			}
		}
		containerRepoPath := r.ContainerPathForHostPath(task, repoHostPath, "")
		repoDiff, diffErr := r.GetPRDiff(ctx, task, agent, containerRepoPath, resolvedBase)
		if diffErr != nil {
			return "", fmt.Errorf("capture PR diff for repo %s: %w", repo.URL, diffErr)
		}
		if strings.TrimSpace(repoDiff) != "" {
			diffOut = append(diffOut, fmt.Sprintf("--- Repository: %s\n%s", repoNameFromURL(repo.URL), repoDiff))
		}
	}
	return strings.Join(diffOut, "\n"), nil
}

func (r *Runner) GetChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	var repos []models.Repository
	var err error
	if r.ListRepositories != nil {
		repos, err = r.ListRepositories(ctx, task.ProjectID)
	}
	if r.ListRepositories == nil || err != nil || len(repos) == 0 {
		containerTargetPath := r.ContainerPathForHostPath(task, targetPath, "")
		return r.SandboxGetChangedFiles(ctx, task, agent, containerTargetPath)
	}

	var targetRepos []models.Repository
	if task.RepositoryID != nil {
		for _, repo := range repos {
			if repo.ID == *task.RepositoryID {
				targetRepos = append(targetRepos, repo)
				break
			}
		}
	} else {
		targetRepos = repos
	}

	ws, errWS := r.LoadTaskWorkspace(ctx, task)

	var allChanged []string
	for _, repo := range targetRepos {
		localRepoPath := targetPath
		prefix := ""
		if errWS == nil {
			for i := range ws.Repos {
				if ws.Repos[i].RepoID == repo.ID {
					if worktreeSuffix != "" {
						role := r.GetRoleFromSuffix(worktreeSuffix)
						if relPath, exists := ws.Repos[i].Paths.Worktrees[role]; exists && relPath != "" {
							localRepoPath = filepath.Join(ws.Root, relPath)
						} else {
							localRepoPath = filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
						}
					} else {
						localRepoPath = filepath.Join(ws.Root, ws.Repos[i].Paths.Main)
					}
					if task.RepositoryID == nil {
						prefix = ws.Repos[i].Name + "/"
					}
					break
				}
			}
		} else if task.RepositoryID == nil {
			parts := strings.Split(repo.URL, "/")
			repoName := parts[len(parts)-1]
			repoName = strings.TrimSuffix(repoName, ".git")
			localRepoPath = filepath.Join(targetPath, repoName)
			prefix = repoName + "/"
		}

		containerRepoPath := r.ContainerPathForHostPath(task, localRepoPath, "")
		repoChanged, err := r.SandboxGetChangedFiles(ctx, task, agent, containerRepoPath)
		if err == nil && len(repoChanged) > 0 {
			for _, line := range repoChanged {
				if line != "" {
					allChanged = append(allChanged, prefix+line)
				}
			}
		}
	}
	return allChanged, nil
}

func IsSafeNewFilePath(path string) bool {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return false
	}
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "../") || strings.Contains(cleaned, "..\\") {
		return false
	}
	parts := strings.Split(cleaned, string(filepath.Separator))
	for _, p := range parts {
		if p == ".git" {
			return false
		}
	}
	return true
}
