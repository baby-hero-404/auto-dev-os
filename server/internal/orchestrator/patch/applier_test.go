package patch

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestRunner_CapturePRDiff_MultiRepo(t *testing.T) {
	task := &models.Task{ID: "task-1", ProjectID: "project-1"}
	ws := &models.TaskWorkspace{
		Root: "/host/workspace/task-1",
		Repos: []models.RepoWorkspace{
			{
				RepoID: "repo-1",
				Name:   "repo-a",
				Paths:  models.RepoWorkspacePaths{Main: filepath.Join("code", "repos", "repo-a", "main")},
			},
			{
				RepoID: "repo-2",
				Name:   "repo-b",
				Paths:  models.RepoWorkspacePaths{Main: filepath.Join("code", "repos", "repo-b", "main")},
			},
		},
	}
	var calledPaths []string
	runner := &Runner{
		WorkspaceRoot: "/host/workspaces",
		ContainerPathForHostPath: func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return strings.Replace(hostPath, "/host/workspace/task-1", "/workspace", 1)
		},
		GetPRDiff: func(ctx context.Context, task *models.Task, agent *models.Agent, containerPath string, baseBranch string) (string, error) {
			calledPaths = append(calledPaths, containerPath)
			return "diff --git a/file.go b/file.go\n", nil
		},
		ListRepositories: func(ctx context.Context, projectID string) ([]models.Repository, error) {
			return []models.Repository{
				{ID: "repo-1", URL: "https://example.com/repo-a.git"},
				{ID: "repo-2", URL: "https://example.com/repo-b.git"},
			}, nil
		},
		LoadTaskWorkspace: func(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
			return ws, nil
		},
	}

	diff, err := runner.CapturePRDiff(context.Background(), task, &models.Agent{}, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calledPaths) != 2 {
		t.Fatalf("expected PR diff for two repos, got %d", len(calledPaths))
	}
	if calledPaths[0] != "/workspace/code/repos/repo-a/main" || calledPaths[1] != "/workspace/code/repos/repo-b/main" {
		t.Fatalf("unexpected called paths: %#v", calledPaths)
	}
	if !strings.Contains(diff, "--- Repository: repo-a") || !strings.Contains(diff, "--- Repository: repo-b") {
		t.Fatalf("expected combined repo headers, got:\n%s", diff)
	}
}
