package models

// Repo merge and test status values
const (
	MergeStatusPending  = "pending"
	MergeStatusMerged   = "merged"
	MergeStatusConflict = "conflict"
	MergeStatusFailed   = "failed"
	MergeStatusSkipped  = "skipped"

	TestStatusPending = "pending"
	TestStatusPassed  = "passed"
	TestStatusFailed  = "failed"
	TestStatusSkipped = "skipped"
)

// TaskWorkspace represents the layout of a task's workspace directories.
type TaskWorkspace struct {
	Root         string          `json:"root"`
	SpecsDir     string          `json:"specs_dir"`
	ContextDir   string          `json:"context_dir"`
	ArtifactsDir string          `json:"artifacts_dir"`
	LogsDir      string          `json:"logs_dir"`
	PRDir        string          `json:"pr_dir"`
	Repos        []RepoWorkspace `json:"repos"`
}

// RepoWorkspaceStatus tracks the integration state of a single repo.
type RepoWorkspaceStatus struct {
	MergeStatus string `json:"merge_status"`
	TestStatus  string `json:"test_status"`
}

// RepoWorkspacePaths defines the relative paths inside the workspace.
type RepoWorkspacePaths struct {
	Main      string            `json:"main"`
	Worktrees map[string]string `json:"worktrees"` // Keyed by role (e.g. "backend", "fix")
}

// RepoWorkspaceBranches defines git branches used for a repo.
type RepoWorkspaceBranches struct {
	Integration string            `json:"integration"`
	Role        map[string]string `json:"role"` // Keyed by role
}

// RepoWorkspace represents the metadata of a repository in the workspace.
type RepoWorkspace struct {
	RepoID        string                `json:"id"`
	Name          string                `json:"name"`
	URL           string                `json:"url"`
	DefaultBranch string                `json:"default_branch"`
	Status        RepoWorkspaceStatus   `json:"status"`
	Paths         RepoWorkspacePaths    `json:"paths"`
	Branches      RepoWorkspaceBranches `json:"branches"`
}

// TaskWorkspaceMetadata represents the metadata.json structure.
type TaskWorkspaceMetadata struct {
	WorkspaceVersion int             `json:"workspace_version"`
	Repos            []RepoWorkspace `json:"repos"`
}

// TaskStateSnapshot represents the task.json structure.
type TaskStateSnapshot struct {
	TaskID      string   `json:"task_id"`
	ProjectID   string   `json:"project_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Complexity  string   `json:"complexity"`
	SpecStatus  string   `json:"spec_status"`
	Labels      []string `json:"labels"`
}
