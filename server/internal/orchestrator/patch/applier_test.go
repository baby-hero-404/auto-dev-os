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
				Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-a", "main")},
			},
			{
				RepoID: "repo-2",
				Name:   "repo-b",
				Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-b", "main")},
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
	if calledPaths[0] != "/workspace/repos/repo-a/main" || calledPaths[1] != "/workspace/repos/repo-b/main" {
		t.Fatalf("unexpected called paths: %#v", calledPaths)
	}
	if !strings.Contains(diff, "--- Repository: repo-a") || !strings.Contains(diff, "--- Repository: repo-b") {
		t.Fatalf("expected combined repo headers, got:\n%s", diff)
	}
}

func TestSplitPatchByRepo(t *testing.T) {
	tests := []struct {
		name     string
		patch    string
		expected map[string]string
	}{
		{
			name: "single repo without prefix",
			patch: `diff --git a/readme.md b/readme.md
--- a/readme.md
+++ b/readme.md
@@ -1,2 +1,2 @@
-hello
+world`,
			expected: map[string]string{
				"": `diff --git a/readme.md b/readme.md
--- a/readme.md
+++ b/readme.md
@@ -1,2 +1,2 @@
-hello
+world`,
			},
		},
		{
			name: "multi repo with prefix",
			patch: `diff --git a/repo-a/src/main.go b/repo-a/src/main.go
--- a/repo-a/src/main.go
+++ b/repo-a/src/main.go
@@ -1,2 +1,2 @@
-a
+b
diff --git a/repo-b/index.js b/repo-b/index.js
--- a/repo-b/index.js
+++ b/repo-b/index.js
@@ -1,2 +1,2 @@
-c
+d`,
			expected: map[string]string{
				"repo-a": `diff --git a/src/main.go b/src/main.go
--- a/src/main.go
+++ b/src/main.go
@@ -1,2 +1,2 @@
-a
+b
`,
				"repo-b": `diff --git a/index.js b/index.js
--- a/index.js
+++ b/index.js
@@ -1,2 +1,2 @@
-c
+d`,
			},
		},
		{
			name: "multi repo container prefix format",
			patch: `diff --git a/code/repos/repo-a/main/src/main.go b/code/repos/repo-a/main/src/main.go
--- a/code/repos/repo-a/main/src/main.go
+++ b/code/repos/repo-a/main/src/main.go
@@ -1,2 +1,2 @@
-1
+2`,
			expected: map[string]string{
				"repo-a": `diff --git a/src/main.go b/src/main.go
--- a/src/main.go
+++ b/src/main.go
@@ -1,2 +1,2 @@
-1
+2`,
			},
		},
		{
			name: "multi repo container prefix format with master branch",
			patch: `diff --git a/code/repos/repo-a/master/src/main.go b/code/repos/repo-a/master/src/main.go
--- a/code/repos/repo-a/master/src/main.go
+++ b/code/repos/repo-a/master/src/main.go
@@ -1,2 +1,2 @@
-1
+2`,
			expected: map[string]string{
				"repo-a": `diff --git a/src/main.go b/src/main.go
--- a/src/main.go
+++ b/src/main.go
@@ -1,2 +1,2 @@
-1
+2`,
			},
		},
		{
			name: "patch without git diff headers - single repo format",
			patch: `--- a/readme.md
+++ b/readme.md
@@ -1,4 +1,4 @@
-# Prompt Base - Test Repository
+# Prompt Base Test Repository`,
			expected: map[string]string{
				"": `--- a/readme.md
+++ b/readme.md
@@ -1,4 +1,4 @@
-# Prompt Base - Test Repository
+# Prompt Base Test Repository`,
			},
		},
		{
			name: "patch without git diff headers - container prefix format",
			patch: `--- a/code/repos/repo-a/main/readme.md
+++ b/code/repos/repo-a/main/readme.md
@@ -1,2 +1,2 @@
-1
+2`,
			expected: map[string]string{
				"repo-a": `--- a/readme.md
+++ b/readme.md
@@ -1,2 +1,2 @@
-1
+2`,
			},
		},
		{
			name: "patch without git diff headers - direct repo prefix format",
			patch: `--- a/repo-a/readme.md
+++ b/repo-a/readme.md
@@ -1,2 +1,2 @@
-1
+2`,
			expected: map[string]string{
				"repo-a": `--- a/readme.md
+++ b/readme.md
@@ -1,2 +1,2 @@
-1
+2`,
			},
		},
		{
			name: "multi repo Option B direct repo prefix format",
			patch: `diff --git a/repo-a/src/main.go b/repo-a/src/main.go
--- a/repo-a/src/main.go
+++ b/repo-a/src/main.go
@@ -1,2 +1,2 @@
-1
+2`,
			expected: map[string]string{
				"repo-a": `diff --git a/src/main.go b/src/main.go
--- a/src/main.go
+++ b/src/main.go
@@ -1,2 +1,2 @@
-1
+2`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := SplitPatchByRepo(tc.patch)
			if len(res) != len(tc.expected) {
				t.Fatalf("expected %d split patches, got %d. Result: %#v", len(tc.expected), len(res), res)
			}
			for k, v := range tc.expected {
				got, ok := res[k]
				if !ok {
					t.Fatalf("expected key %q in result, got result: %#v", k, res)
				}
				if strings.TrimSpace(got) != strings.TrimSpace(v) {
					t.Errorf("key %q: expected:\n%q\ngot:\n%q", k, v, got)
				}
			}
		})
	}
}
