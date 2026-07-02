package workspace

import (
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

// ReposDirName is the folder name that holds all cloned repositories for a task.
const ReposDirName = "code/repos"

// ReposPrefix returns the prefix string for repository paths (e.g. "code/repos/").
func ReposPrefix() string {
	return ReposDirName + "/"
}

// PathManager centralises all workspace directory and path resolution logic
// for a task. It serves as a single source of truth so that callers never have
// to manually construct paths.
type PathManager struct {
	workspaceRoot string
}

// NewPathManager creates a PathManager rooted at workspaceRoot.
// workspaceRoot is the base directory that holds all task workspaces
// (e.g. "server/.data/workspaces" or the value of SANDBOX_WORKSPACE_ROOT).
func NewPathManager(workspaceRoot string) *PathManager {
	return &PathManager{workspaceRoot: workspaceRoot}
}

// ── Task-level paths ─────────────────────────────────────────────

// TaskRoot returns the absolute path to the root of a task workspace.
func (m *PathManager) TaskRoot(taskID string) string {
	return sandbox.WorkspacePath(m.workspaceRoot, taskID)
}

// SpecsDir returns the path for requirement specification files.
func (m *PathManager) SpecsDir(taskID string) string {
	return filepath.Join(m.TaskRoot(taskID), "specs")
}

// ContextDir returns the path for loaded context data.
func (m *PathManager) ContextDir(taskID string) string {
	return filepath.Join(m.TaskRoot(taskID), "context")
}

// ArtifactsDir returns the path for generated artefacts (diffs, checkpoints, tests).
func (m *PathManager) ArtifactsDir(taskID string) string {
	return filepath.Join(m.TaskRoot(taskID), "artifacts")
}

// LogsDir returns the path for task-level logs.
func (m *PathManager) LogsDir(taskID string) string {
	return filepath.Join(m.TaskRoot(taskID), "logs")
}

// PRDir returns the path for pull-request related files.
func (m *PathManager) PRDir(taskID string) string {
	return filepath.Join(m.TaskRoot(taskID), "pr")
}

// OpenSpecDir returns the path for an OpenSpec change set.
func (m *PathManager) OpenSpecDir(taskID, changeName string) string {
	return filepath.Join(m.TaskRoot(taskID), "openspec", "changes", changeName)
}

// ── Repository paths ─────────────────────────────────────────────

// CodeRoot returns the directory that holds all cloned repositories for a task.
func (m *PathManager) CodeRoot(taskID string) string {
	return filepath.Join(m.TaskRoot(taskID), ReposDirName)
}

// RepoRoot returns the root directory for a specific repository within the task.
func (m *PathManager) RepoRoot(taskID, repoName string) string {
	return filepath.Join(m.CodeRoot(taskID), repoName)
}

// RepoMain returns the main integration checkout of a repository.
// This corresponds to RepoWorkspacePaths.Main ("code/repos/<name>/<branch>").
func (m *PathManager) RepoMain(taskID, repoName, branch string) string {
	if branch == "" {
		branch = "main"
	}
	return filepath.Join(m.RepoRoot(taskID, repoName), branch)
}

// RepoMainRelative returns the workspace-relative path to the main checkout
// (e.g. "code/repos/test/main"). This is the value stored in metadata.json.
func (m *PathManager) RepoMainRelative(repoName, branch string) string {
	if branch == "" {
		branch = "main"
	}
	return filepath.Join(ReposDirName, repoName, branch)
}

// RepoWorktreeDir returns the directory for a role-specific git worktree.
func (m *PathManager) RepoWorktreeDir(taskID, repoName, role string) string {
	return filepath.Join(m.RepoRoot(taskID, repoName), "worktrees", role)
}

// RepoWorktreeRelative returns the workspace-relative path to a role worktree.
func (m *PathManager) RepoWorktreeRelative(repoName, role string) string {
	return filepath.Join(ReposDirName, repoName, "worktrees", role)
}

// ── Path translation ─────────────────────────────────────────────

func WorkspaceToRepoRelative(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))

	prefix := "/code/repos/"
	idx := strings.Index(path, prefix)
	if idx == -1 {
		if strings.HasPrefix(path, "code/repos/") {
			idx = 0
			path = "/" + path
		} else {
			return path
		}
	}

	after := path[idx+len(prefix):]
	parts := strings.Split(after, "/")
	if len(parts) < 2 {
		return strings.TrimPrefix(path, "/")
	}

	repoName := parts[0]
	if parts[1] == "worktrees" {
		if len(parts) >= 4 {
			return repoName + "/" + strings.Join(parts[3:], "/")
		}
		return strings.TrimPrefix(path, "/")
	}

	if len(parts) >= 3 {
		return repoName + "/" + strings.Join(parts[2:], "/")
	}

	return strings.TrimPrefix(path, "/")
}

// RepoRelativeToWorkspace converts a repo-relative path (e.g. "readme.md" or "test/readme.md")
// into a workspace-relative path under the given repo's main checkout
// (e.g. "code/repos/test/main/readme.md").
func RepoRelativeToWorkspace(repoName, branch, repoPath string) string {
	if branch == "" {
		branch = "main"
	}
	repoPath = filepath.ToSlash(filepath.Clean(repoPath))
	repoPrefix := repoName + "/"
	if strings.HasPrefix(repoPath, repoPrefix) {
		repoPath = repoPath[len(repoPrefix):]
	} else if repoPath == repoName {
		repoPath = ""
	}
	return filepath.ToSlash(filepath.Join(ReposDirName, repoName, branch, repoPath))
}

// IsWorkspaceInternalPath returns true if the path points to a workspace
// meta-directory (artifacts, logs, specs, openspec, context, pr) rather
// than actual source code.
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
