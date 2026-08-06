package wkspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// fakeGitOpsLocalClone shells out to real `git clone` against a local bare
// repo path so EnsureWorkspaceCloned exercises the actual clone/reset code
// path in this test, without touching the network.
type fakeGitOpsLocalClone struct {
	remoteRepoPath string
}

func (f *fakeGitOpsLocalClone) CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, f.remoteRepoPath, localPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %v: %s", err, string(out))
	}
	return branch, nil
}

func (f *fakeGitOpsLocalClone) TokenForRepoURL(ctx context.Context, repoURL string) (string, error) {
	return "", nil
}

// fakeTasksRepoByID is a TaskRepository backed by an in-memory map, needed
// (unlike the package's testMockTasksRepo, which always returns nil, nil)
// so ReleaseWorkspaceLock can actually resolve a child's parent via
// models.WorkspaceOwnerID in this test.
type fakeTasksRepoByID struct {
	tasks map[string]*models.Task
}

func (f *fakeTasksRepoByID) GetByID(ctx context.Context, id string) (*models.Task, error) {
	if task, ok := f.tasks[id]; ok {
		return task, nil
	}
	return nil, fmt.Errorf("task %s not found", id)
}

// TestDecomposedChildren_ShareWorkspaceAndSeeEachOthersCommits is the
// concrete regression test for the fix described in
// docs/openspecs/task-subtask-decomposition/design.md ("Key Decisions":
// children execute sequentially on the same branch/worktree as the parent,
// each committing its own changes before the next child starts).
//
// It drives two sequential children of the same parent through
// EnsureWorkspaceCloned exactly as dispatchNextChildOrReduce
// (orchestrator/decomposition.go) does for each child in turn, committing
// child 1's change directly into the shared repo checkout in between, and
// asserts:
//  1. Both children resolve to the identical on-disk repo path (same
//     workspace, keyed by the parent's ID via models.WorkspaceOwnerID).
//  2. Child 2's checkout contains child 1's committed file — i.e. child 2
//     is NOT a fresh clone of the base branch, it continues on top of
//     child 1's work.
func TestDecomposedChildren_ShareWorkspaceAndSeeEachOthersCommits(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "decomp-ws-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up a local "remote" bare-ish repo with one commit on main.
	remotePath := filepath.Join(tmpDir, "remote-repo")
	if err := os.MkdirAll(remotePath, 0o755); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}
	runGit(t, remotePath, "init", "-b", "main")
	runGit(t, remotePath, "config", "user.email", "test@example.com")
	runGit(t, remotePath, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(remotePath, "README.md"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, remotePath, "add", ".")
	runGit(t, remotePath, "commit", "-m", "initial commit")

	parentID := "parent-task-1"
	repoID := "repo-1"
	repos := []models.Repository{
		{ID: repoID, ProjectID: "proj-1", URL: remotePath, Branch: "main"},
	}

	child1 := &models.Task{
		ID:            "child-1",
		ParentTaskID:  &parentID,
		SequenceIndex: intPtr(0),
		ProjectID:     "proj-1",
		RepositoryID:  &repoID,
		Title:         "Child 1: add feature A",
		Status:        "coding",
	}
	child2 := &models.Task{
		ID:            "child-2",
		ParentTaskID:  &parentID,
		SequenceIndex: intPtr(1),
		ProjectID:     "proj-1",
		RepositoryID:  &repoID,
		Title:         "Child 2: add feature B",
		Status:        "coding",
	}

	workspaceRoot := filepath.Join(tmpDir, "workspaces")
	manager := NewManager(
		&fakeTasksRepoByID{tasks: map[string]*models.Task{child1.ID: child1, child2.ID: child2}},
		&testMockWorkflowsRepo{},
		&testMockRepositoriesRepo{repos: repos},
		&fakeGitOpsLocalClone{remoteRepoPath: remotePath},
		nil,
		workspaceRoot,
		WorkspaceRetention{},
		nil,
		nil,
	)

	ctx := context.Background()

	if err := manager.EnsureWorkspaceCloned(ctx, child1, &models.Agent{ID: "agent-1"}, "job-1"); err != nil {
		t.Fatalf("EnsureWorkspaceCloned(child1) failed: %v", err)
	}
	manager.ReleaseWorkspaceLock(ctx, child1.ID)

	ws1, err := manager.LoadTaskWorkspace(ctx, child1)
	if err != nil {
		t.Fatalf("LoadTaskWorkspace(child1) failed: %v", err)
	}
	repoPath1 := filepath.Join(ws1.Root, ws1.Repos[0].Paths.Main)

	// Simulate child 1 committing its own change directly into the shared
	// checkout, exactly as CommitRoleWorktrees/CreateGitCheckpoint would.
	if err := os.WriteFile(filepath.Join(repoPath1, "feature-a.txt"), []byte("feature A"), 0o644); err != nil {
		t.Fatalf("write feature-a.txt: %v", err)
	}
	runGit(t, repoPath1, "config", "user.email", "agent@example.com")
	runGit(t, repoPath1, "config", "user.name", "Agent")
	runGit(t, repoPath1, "add", ".")
	runGit(t, repoPath1, "commit", "-m", "child 1: add feature A")

	// Dispatch child 2 exactly as dispatchNextChildOrReduce does.
	if err := manager.EnsureWorkspaceCloned(ctx, child2, &models.Agent{ID: "agent-1"}, "job-2"); err != nil {
		t.Fatalf("EnsureWorkspaceCloned(child2) failed: %v", err)
	}
	manager.ReleaseWorkspaceLock(ctx, child2.ID)

	ws2, err := manager.LoadTaskWorkspace(ctx, child2)
	if err != nil {
		t.Fatalf("LoadTaskWorkspace(child2) failed: %v", err)
	}
	repoPath2 := filepath.Join(ws2.Root, ws2.Repos[0].Paths.Main)

	if repoPath1 != repoPath2 {
		t.Fatalf("expected child1 and child2 to resolve to the same repo path, got %q vs %q", repoPath1, repoPath2)
	}
	if ws1.Root != ws2.Root {
		t.Fatalf("expected child1 and child2 to share the same workspace root, got %q vs %q", ws1.Root, ws2.Root)
	}

	// The concrete assertion this fix must produce: child 2's checkout
	// contains child 1's committed file.
	featureAPath := filepath.Join(repoPath2, "feature-a.txt")
	content, err := os.ReadFile(featureAPath)
	if err != nil {
		t.Fatalf("expected child2's workspace to contain child1's committed feature-a.txt, but got error: %v", err)
	}
	if string(content) != "feature A" {
		t.Errorf("expected feature-a.txt content %q, got %q", "feature A", string(content))
	}

	// And the workspace-owning branch name is shared/consistent, not
	// recomputed per-child with a mismatched slug.
	if ws1.Repos[0].Branches.Integration != ws2.Repos[0].Branches.Integration {
		t.Errorf("expected shared integration branch name, got %q vs %q",
			ws1.Repos[0].Branches.Integration, ws2.Repos[0].Branches.Integration)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, string(out))
	}
}

func intPtr(i int) *int { return &i }
