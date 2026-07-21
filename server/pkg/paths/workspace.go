package paths

import (
	"os"
	"path/filepath"
	"strings"
)

// ReposDirName is the folder name that holds all cloned repositories for a task.
const ReposDirName = "code/repos"

// ReposPrefix returns the prefix string for repository paths (e.g. "code/repos/").
func ReposPrefix() string {
	return ReposDirName + "/"
}

// OSWorkspacePaths implements WorkspacePaths for the local OS filesystem.
type OSWorkspacePaths struct {
	workspaceRoot string
}

// NewOSWorkspacePaths creates a new OSWorkspacePaths provider.
func NewOSWorkspacePaths(workspaceRoot string) *OSWorkspacePaths {
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(os.TempDir(), "auto-code-os", "workspaces")
	}
	if absRoot, err := filepath.Abs(workspaceRoot); err == nil {
		workspaceRoot = absRoot
	}
	return &OSWorkspacePaths{workspaceRoot: filepath.ToSlash(filepath.Clean(workspaceRoot))}
}

// TaskRoot returns the absolute path to the root of a task workspace.
func (w *OSWorkspacePaths) TaskRoot(taskID string) Directory {
	return NewDirectory(filepath.Join(w.workspaceRoot, taskID))
}

// SpecsDir returns the path for requirement specification files.
func (w *OSWorkspacePaths) SpecsDir(taskID string) Directory {
	return w.TaskRoot(taskID).Child("specs")
}

// ContextDir returns the path for loaded context data.
func (w *OSWorkspacePaths) ContextDir(taskID string) Directory {
	return w.TaskRoot(taskID).Child("context")
}

// ArtifactsDir returns the path for generated artifacts (diffs, checkpoints, tests).
func (w *OSWorkspacePaths) ArtifactsDir(taskID string) Directory {
	return w.TaskRoot(taskID).Child("artifacts")
}

// LogsDir returns the path for task-level logs.
func (w *OSWorkspacePaths) LogsDir(taskID string) Directory {
	return w.TaskRoot(taskID).Child("logs")
}

// PRDir returns the path for pull-request related files.
func (w *OSWorkspacePaths) PRDir(taskID string) Directory {
	return w.TaskRoot(taskID).Child("pr")
}

// OpenSpecDir returns the path for an OpenSpec change set.
func (w *OSWorkspacePaths) OpenSpecDir(taskID, changeName string) Directory {
	return w.TaskRoot(taskID).Child("openspec", "changes", changeName)
}

// CodeRoot returns the directory that holds all cloned repositories for a task.
func (w *OSWorkspacePaths) CodeRoot(taskID string) Directory {
	return w.TaskRoot(taskID).Child(strings.Split(ReposDirName, "/")...)
}

// RepoRoot returns the root directory for a specific repository within the task.
func (w *OSWorkspacePaths) RepoRoot(taskID, repoName string) Directory {
	return w.CodeRoot(taskID).Child(repoName)
}

// RepoMain returns the main integration checkout of a repository.
func (w *OSWorkspacePaths) RepoMain(taskID, repoName string) Directory {
	return w.RepoRoot(taskID, repoName).Child("main")
}

// RepoMainRelative returns the workspace-relative path to the main checkout.
func (w *OSWorkspacePaths) RepoMainRelative(repoName string) string {
	return filepath.ToSlash(filepath.Join(ReposDirName, repoName, "main"))
}

// RepoWorktreeDir returns the directory for a role-specific git worktree.
func (w *OSWorkspacePaths) RepoWorktreeDir(taskID, repoName, role string) Directory {
	return w.RepoRoot(taskID, repoName).Child("worktrees", role)
}

// RepoWorktreeRelative returns the workspace-relative path to a role worktree.
func (w *OSWorkspacePaths) RepoWorktreeRelative(repoName, role string) string {
	return filepath.ToSlash(filepath.Join(ReposDirName, repoName, "worktrees", role))
}

// WorkspaceToRepoRelative converts a workspace-relative path under code/repos/ into a repo-relative path (e.g. "repoName/readme.md").
func WorkspaceToRepoRelative(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))

	prefix := "/" + ReposPrefix()
	idx := strings.Index(path, prefix)
	if idx == -1 {
		if strings.HasPrefix(path, ReposPrefix()) {
			idx = 0
			path = "/" + path
		} else {
			return path
		}
	}

	after := path[idx+len(prefix):]
	parts := strings.Split(after, "/")
	if len(parts) < 2 {
		if len(parts) == 1 && parts[0] != "" {
			return parts[0]
		}
		return strings.TrimPrefix(path, "/")
	}

	repoName := parts[0]
	if parts[1] == "worktrees" {
		if len(parts) >= 4 {
			return repoName + "/" + strings.Join(parts[3:], "/")
		}
		return repoName
	}

	if len(parts) >= 3 {
		return repoName + "/" + strings.Join(parts[2:], "/")
	}

	return repoName
}

// RepoRelativeToWorkspace converts a repo-relative path (e.g. "readme.md") into a workspace-relative path (e.g. "code/repos/repoName/main/readme.md").
func RepoRelativeToWorkspace(repoName, repoPath string) string {
	repoPath = filepath.ToSlash(filepath.Clean(repoPath))
	repoPrefix := repoName + "/"
	if strings.HasPrefix(repoPath, repoPrefix) {
		repoPath = repoPath[len(repoPrefix):]
	} else if repoPath == repoName {
		repoPath = ""
	}
	return filepath.ToSlash(filepath.Join(ReposDirName, repoName, "main", repoPath))
}

// IsWorkspaceInternalPath returns true if the path points to a workspace meta-directory (artifacts, logs, specs, openspec, context, pr).
func IsWorkspaceInternalPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, dir := range []string{
		"artifacts/", "logs/", "specs/", "openspec/", "context/", "pr/",
	} {
		if strings.HasPrefix(path, dir) || path == strings.TrimSuffix(dir, "/") {
			return true
		}
	}
	return false
}
