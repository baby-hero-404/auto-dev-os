package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	isRestore := strings.HasSuffix(stepID, "_restore")
	allowedNewFiles := make(map[string]bool)
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
				paths.QuoteShellArg(containerTargetPath),
				paths.QuoteShellArg(containerPatchPath),
			)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch", revertCmd, worktreeSuffix)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
			return fmt.Errorf("git apply patch: %w", err)
		}
		if len(allowedNewFiles) > 0 {
			if err := r.appendNewAffectedFiles(ctx, task, allowedNewFiles); err != nil && r.Log != nil {
				r.Log(ctx, task.ID, "warn", fmt.Sprintf("failed to persist new affected files: %v", err))
			}
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
			// Fallback to ReposPrefix + repoName/<defaultBranch>
			repoDir := paths.NewOSWorkspacePaths(r.WorkspaceRoot).RepoRoot(task.ID, repoName).String()
			mainDirName := paths.FindRepoMainBranchDir(repoDir)
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
				paths.QuoteShellArg(containerRepoWorktreePath),
				paths.QuoteShellArg(containerPatchPath),
			)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch_"+repoName, revertCmd, worktreeSuffix)
			_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
			return fmt.Errorf("git apply patch failed for repo %s: %w", repoName, err)
		}
		if len(allowedNewFiles) > 0 {
			if err := r.appendNewAffectedFiles(ctx, task, allowedNewFiles); err != nil && r.Log != nil {
				r.Log(ctx, task.ID, "warn", fmt.Sprintf("failed to persist new affected files: %v", err))
			}
		}
		_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm -f %s", paths.QuoteShellArg(containerPatchPath)), worktreeSuffix)
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
		existing[file.File] = true
	}
	for file := range files {
		if !existing[file] {
			analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{File: file})
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
			repoHostPath = paths.NewOSWorkspacePaths(r.WorkspaceRoot).RepoMain(task.ID, repoName, "").String()
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

func (r *Runner) NormalizePatchPath(firstPath string, ws *models.TaskWorkspace, role string) (repoName string, repoRelPath string) {
	if firstPath == "" {
		return "", ""
	}
	// Clean and normalize separators
	firstPath = strings.ReplaceAll(firstPath, "\\", "/")
	firstPath = filepath.ToSlash(filepath.Clean(firstPath))
	firstPath = strings.TrimPrefix(firstPath, "/")

	// Strip git diff a/ or b/ prefixes if present
	if strings.HasPrefix(firstPath, "a/") || strings.HasPrefix(firstPath, "b/") {
		firstPath = firstPath[2:]
	}

	// Helper to check and strip prefix
	stripPrefix := func(p string, prefix string) (string, bool) {
		if p == prefix {
			return "", true
		}
		if strings.HasPrefix(p, prefix+"/") {
			return p[len(prefix)+1:], true
		}
		return "", false
	}

	// 1. Check if it starts with code/repos/ prefix
	if rem, ok := stripPrefix(firstPath, paths.ReposDirName); ok {
		firstPath = rem
	}

	// 2. Try to match against workspace repository names
	if ws != nil {
		for _, repo := range ws.Repos {
			if rem, ok := stripPrefix(firstPath, repo.Name); ok {
				// Strip worktrees/<role> or worktrees/<any_role> or main branch dir if present
				if rem2, ok2 := stripPrefix(rem, "worktrees/"+role); ok2 {
					return repo.Name, rem2
				}
				if strings.HasPrefix(rem, "worktrees/") {
					parts := strings.Split(rem, "/")
					if len(parts) >= 3 {
						return repo.Name, strings.Join(parts[2:], "/")
					}
				}
				
				mainBranchDir := "main"
				if repo.Paths.Main != "" {
					mainBranchDir = filepath.Base(repo.Paths.Main)
				} else if repo.DefaultBranch != "" {
					mainBranchDir = repo.DefaultBranch
				}

				if rem2, ok2 := stripPrefix(rem, mainBranchDir); ok2 {
					return repo.Name, rem2
				}
				if mainBranchDir != "main" {
					if rem2, ok2 := stripPrefix(rem, "main"); ok2 {
						return repo.Name, rem2
					}
				}
				return repo.Name, rem
			}
		}
	}

	// 3. Fallback: if we only have 1 repository in the workspace, it must belong to it!
	if ws != nil && len(ws.Repos) == 1 {
		repo := ws.Repos[0]
		// Still check if the path starts with worktrees/role or role to clean it
		rem := firstPath
		if rem2, ok2 := stripPrefix(rem, "worktrees/"+role); ok2 {
			return repo.Name, rem2
		}
		if strings.HasPrefix(rem, "worktrees/") {
			parts := strings.Split(rem, "/")
			if len(parts) >= 3 {
				return repo.Name, strings.Join(parts[2:], "/")
			}
		}
		if rem2, ok2 := stripPrefix(rem, role); ok2 {
			return repo.Name, rem2
		}

		mainBranchDir := "main"
		if repo.Paths.Main != "" {
			mainBranchDir = filepath.Base(repo.Paths.Main)
		} else if repo.DefaultBranch != "" {
			mainBranchDir = repo.DefaultBranch
		}
		if rem2, ok2 := stripPrefix(rem, mainBranchDir); ok2 {
			return repo.Name, rem2
		}
		if mainBranchDir != "main" {
			if rem2, ok2 := stripPrefix(rem, "main"); ok2 {
				return repo.Name, rem2
			}
		}

		return repo.Name, rem
	}

	// 4. Fallback: parse first component as repoName
	idx := strings.Index(firstPath, "/")
	if idx != -1 {
		return firstPath[:idx], firstPath[idx+1:]
	}
	return "", firstPath
}

func (r *Runner) CleanPatchBlock(block string, repoName string, repoRelPath string, rawFirstPath string) string {
	if !strings.HasSuffix(rawFirstPath, repoRelPath) {
		return CleanRepoPrefix(block, repoName)
	}
	prefixToStrip := rawFirstPath[:len(rawFirstPath)-len(repoRelPath)]
	if prefixToStrip == "" {
		return block
	}

	escapedPrefix := regexp.QuoteMeta(prefixToStrip)
	block = regexp.MustCompile(`^(a/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`( b/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(--- a/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(\+\+\+ b/)`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(rename from )`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(rename to )`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(copy from )`+escapedPrefix).ReplaceAllString(block, "${1}")
	block = regexp.MustCompile(`(?m)^(copy to )`+escapedPrefix).ReplaceAllString(block, "${1}")
	return block
}

func (r *Runner) SplitPatchByRepoWithWorkspace(patchText string, ws *models.TaskWorkspace, role string) map[string]string {
	repos := make(map[string]string)
	parts := strings.Split(patchText, "diff --git ")
	if len(parts) <= 1 || (len(parts) == 2 && parts[0] == "" && !strings.Contains(patchText, "diff --git ")) {
		trimmed := strings.TrimSpace(patchText)
		if trimmed == "" {
			return repos
		}
		lines := strings.Split(trimmed, "\n")
		var firstPath string
		for _, line := range lines {
			if strings.HasPrefix(line, "--- a/") {
				firstPath = line[len("--- a/"):]
				break
			} else if strings.HasPrefix(line, "+++ b/") {
				firstPath = line[len("+++ b/"):]
				break
			}
		}
		if firstPath != "" {
			repoName, repoRelPath := r.NormalizePatchPath(firstPath, ws, role)
			if repoName != "" {
				cleanPatch := r.CleanPatchBlock(trimmed, repoName, repoRelPath, firstPath)
				repos[repoName] = cleanPatch
			} else {
				repos[""] = trimmed
			}
		} else {
			repos[""] = trimmed
		}
		return repos
	}

	header := parts[0]
	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		lineEnd := strings.Index(block, "\n")
		if lineEnd == -1 {
			lineEnd = len(block)
		}
		headerLine := block[:lineEnd]

		var firstPath string
		if strings.HasPrefix(headerLine, "a/") {
			sub := headerLine[2:]
			sepIdx := strings.Index(sub, " b/")
			if sepIdx != -1 {
				firstPath = sub[:sepIdx]
			}
		}

		if firstPath != "" {
			repoName, repoRelPath := r.NormalizePatchPath(firstPath, ws, role)
			if repoName != "" {
				cleanBlock := r.CleanPatchBlock(block, repoName, repoRelPath, firstPath)
				repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+cleanBlock)
			} else {
				repoBlocks[""] = append(repoBlocks[""], "diff --git "+block)
			}
		} else {
			repoBlocks[""] = append(repoBlocks[""], "diff --git "+block)
		}
	}

	for repoName, blocks := range repoBlocks {
		repos[repoName] = header + strings.Join(blocks, "")
	}
	return repos
}
