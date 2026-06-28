package workspace

import (
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

// PathManager centralises all workspace directory and path resolution logic
// for a task. It serves as a single source of truth so that callers never have
// to manually construct paths like filepath.Join("code","repos",name,"main").
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
	return filepath.Join(m.TaskRoot(taskID), "code", "repos")
}

// RepoRoot returns the root directory for a specific repository within the task.
func (m *PathManager) RepoRoot(taskID, repoName string) string {
	return filepath.Join(m.CodeRoot(taskID), repoName)
}

// RepoMain returns the main integration checkout of a repository.
// This corresponds to RepoWorkspacePaths.Main ("code/repos/<name>/main").
func (m *PathManager) RepoMain(taskID, repoName string) string {
	return filepath.Join(m.RepoRoot(taskID, repoName), "main")
}

// RepoMainRelative returns the workspace-relative path to the main checkout
// (e.g. "code/repos/test/main"). This is the value stored in metadata.json.
func (m *PathManager) RepoMainRelative(repoName string) string {
	return filepath.Join("code", "repos", repoName, "main")
}

// RepoWorktreeDir returns the directory for a role-specific git worktree.
func (m *PathManager) RepoWorktreeDir(taskID, repoName, role string) string {
	return filepath.Join(m.RepoRoot(taskID, repoName), "worktrees", role)
}

// RepoWorktreeRelative returns the workspace-relative path to a role worktree.
func (m *PathManager) RepoWorktreeRelative(repoName, role string) string {
	return filepath.Join("code", "repos", repoName, "worktrees", role)
}

// ── Path translation ─────────────────────────────────────────────

// WorkspaceToRepoRelative converts a workspace-relative path (e.g.
// "code/repos/test/main/readme.md") into a repo-relative path
// ("readme.md"). This is the key function that prevents the LLM from
// seeing internal workspace layout in prompts.
//
// If the path does not match any known repo prefix pattern it is
// returned unchanged.
func WorkspaceToRepoRelative(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))

	const prefix = "code/repos/"
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	after := path[len(prefix):] // e.g. "test/main/readme.md" or "test/worktrees/backend/src/main.go"
	parts := strings.Split(after, "/")
	if len(parts) < 3 {
		return path
	}
	// parts[0] = repo name, parts[1] = main/default checkout or "worktrees"
	if parts[1] == "worktrees" && len(parts) >= 4 {
		// parts[2] = worktree role (e.g. "backend")
		return strings.Join(parts[3:], "/")
	}
	if parts[1] != "worktrees" {
		return strings.Join(parts[2:], "/")
	}
	return path
}

// RepoRelativeToWorkspace converts a repo-relative path (e.g. "readme.md")
// into a workspace-relative path under the given repo's main checkout
// (e.g. "code/repos/test/main/readme.md").
func RepoRelativeToWorkspace(repoName, repoPath string) string {
	repoPath = filepath.ToSlash(filepath.Clean(repoPath))
	return filepath.ToSlash(filepath.Join("code", "repos", repoName, "main", repoPath))
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
