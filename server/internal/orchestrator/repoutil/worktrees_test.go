package repoutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// newTestManager builds a Manager whose sandbox execution runs bash directly against a
// local temp git repo, bypassing the container runtime entirely. This exercises the exact
// git script CreateGitCheckpoint/RestoreGitCheckpoint run, not the sandboxing plumbing.
func newTestManager(t *testing.T, repoDir string) (*Manager, *models.Task, *models.Repository, *models.Agent) {
	t.Helper()

	repo := models.Repository{ID: "repo-1", URL: "https://example.com/org/repo-1.git"}
	task := &models.Task{ID: "task-1", ProjectID: "proj-1"}
	agent := &models.Agent{ID: "agent-1", Name: "Test Agent"}

	ws := &models.TaskWorkspace{
		Root: "",
		Repos: []models.RepoWorkspace{
			{
				RepoID: repo.ID,
				Paths:  models.RepoWorkspacePaths{Main: repoDir},
			},
		},
	}

	m := &Manager{
		WorkspaceRoot: t.TempDir(),
		ListRepositories: func(ctx context.Context, projectID string) ([]models.Repository, error) {
			return []models.Repository{repo}, nil
		},
		GetTaskWorkspace: func(task *models.Task) *models.TaskWorkspace { return ws },
		ContainerPathForHostPath: func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return hostPath
		},
		RunSandboxStepInWorktree: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, script string, suffix string) (map[string]any, error) {
			cmd := exec.CommandContext(ctx, "bash", "-c", script)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, err
			}
			return map[string]any{"status": "ok", "stdout": string(out)}, nil
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level string, message string) {
			t.Logf("[%s] %s", level, message)
		},
		DefaultAgentName:  "Test Agent",
		DefaultAgentEmail: "test-agent@autocode.os",
	}

	return m, task, &repo, agent
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/repo\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "go.mod")
	run("commit", "-q", "-m", "initial commit")
}

// TestCreateGitCheckpoint_CapturesNewFileInNewDirectory reproduces the real-world data-loss
// bug traced from task d460fb94-cba4-46ae-bedd-bef5d7dd734e: an LLM step creates the first
// file in a brand-new subdirectory (e.g. internal/repository/sqlite.go). git status --porcelain
// collapses an entirely-untracked directory into a single "?? internal/" line, so a checkpoint
// script that stages by walking that output and matching per-file extensions never sees a
// "internal/repository/sqlite.go" line to match against — the whole directory is silently
// skipped, `--allow-empty` lets the commit "succeed" anyway, and a later RestoreGitCheckpoint's
// `git clean -fd` permanently deletes the never-committed file.
func TestCreateGitCheckpoint_CapturesNewFileInNewDirectory(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	m, task, _, agent := newTestManager(t, repoDir)

	// Simulate the LLM's create_file tool call: a new file in a brand-new subdirectory.
	newDir := filepath.Join(repoDir, "internal", "repository")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "package repository\n\nimport _ \"github.com/mattn/go-sqlite3\"\n"
	if err := os.WriteFile(filepath.Join(newDir, "sqlite.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	hash, err := m.CreateGitCheckpoint(context.Background(), task, agent, "code_backend_0", "")
	if err != nil {
		t.Fatalf("CreateGitCheckpoint failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a non-empty commit hash")
	}

	// The regression: verify the new file actually made it into the checkpoint commit's tree,
	// not just that some commit was created (that part always "succeeded" even with the bug,
	// via --allow-empty).
	cmd := exec.Command("git", "show", "--name-only", "--pretty=format:", hash)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git show failed: %v\n%s", err, out)
	}
	files := strings.Fields(string(out))
	found := false
	for _, f := range files {
		if f == "internal/repository/sqlite.go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("checkpoint commit %s does not contain internal/repository/sqlite.go; committed files: %v", hash, files)
	}

	// And it must survive a restore-to-self: RestoreGitCheckpoint's `git clean -fd` should not
	// delete anything, because the file is now tracked/committed rather than a bystander
	// untracked directory.
	if err := m.RestoreGitCheckpoint(context.Background(), task, agent, hash, ""); err != nil {
		t.Fatalf("RestoreGitCheckpoint failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "sqlite.go")); err != nil {
		t.Fatalf("sqlite.go did not survive restore-to-self: %v", err)
	}
}

// TestCreateGitCheckpoint_EmptyCheckpointIsLogged ensures a genuinely empty step (no changes
// at all) still succeeds as an empty commit, but now surfaces a warning instead of silently
// reporting success with zero captured content.
func TestCreateGitCheckpoint_EmptyCheckpointIsLogged(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	m, task, _, agent := newTestManager(t, repoDir)

	var warnings []string
	m.Log = func(ctx context.Context, taskID string, jobID *string, level string, message string) {
		if level == "warn" {
			warnings = append(warnings, message)
		}
	}

	hash, err := m.CreateGitCheckpoint(context.Background(), task, agent, "noop_step", "")
	if err != nil {
		t.Fatalf("CreateGitCheckpoint failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a non-empty commit hash even for an empty checkpoint")
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning to be logged for an empty checkpoint, got none")
	}
}

// TestRestoreGitCheckpoint_DoesNotDiscardProgressAheadOfCheckpoint reproduces the data-loss bug
// traced from task cdf739c8-bd39-4d11-8633-f9b2ca804637: a review-fix loop-back re-enters
// worker.go's job resume path, which restores the worktree to the last checkpoint with
// status=="success" (e.g. the last code_backend_N step). A "fix" step's salvaged partial edits
// land in a further commit on top of that checkpoint but never earn their own "success" status
// checkpoint, so on the very next loop-back the restore call would reset straight back past
// them — silently discarding every fix attempt's work, every single cycle. RestoreGitCheckpoint
// must be a no-op when the worktree already contains (is a descendant of) the checkpoint commit.
func TestRestoreGitCheckpoint_DoesNotDiscardProgressAheadOfCheckpoint(t *testing.T) {
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	m, task, _, agent := newTestManager(t, repoDir)

	// Checkpoint A: simulates the last successful code_backend_N step.
	checkpointHash, err := m.CreateGitCheckpoint(context.Background(), task, agent, "code_backend_3", "")
	if err != nil {
		t.Fatalf("CreateGitCheckpoint (checkpoint) failed: %v", err)
	}

	// Simulate a "fix" step's salvaged edit landing in a further commit on top of the checkpoint.
	salvagedFile := filepath.Join(repoDir, "cmd", "sync-service", "main.go")
	if err := os.MkdirAll(filepath.Dir(salvagedFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(salvagedFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateGitCheckpoint(context.Background(), task, agent, "fix_salvage", ""); err != nil {
		t.Fatalf("CreateGitCheckpoint (salvage) failed: %v", err)
	}

	// A review-fix loop-back resumes the job, which restores to the last *successful* checkpoint
	// (checkpointHash) — this must NOT wipe out the salvaged main.go committed after it.
	if err := m.RestoreGitCheckpoint(context.Background(), task, agent, checkpointHash, ""); err != nil {
		t.Fatalf("RestoreGitCheckpoint failed: %v", err)
	}

	if _, err := os.Stat(salvagedFile); err != nil {
		t.Fatalf("salvaged file was discarded by restoring to an earlier checkpoint it's already ahead of: %v", err)
	}
}
