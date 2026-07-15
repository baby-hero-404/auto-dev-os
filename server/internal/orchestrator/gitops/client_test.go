package gitops

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func runCmd(t *testing.T, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to run %s %v: %v\nOutput: %s", name, args, err, string(out))
	}
}

func TestGetWorkspaceDiff_MultiRepo(t *testing.T) {
	// Create a temp directory simulating the container path (workspace root)
	tmpDir, err := ioutil.TempDir("", "test-workspace-diff-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directories: code/repos/x/main
	repoRelPath := "code/repos/x/main"
	repoFullDir := filepath.Join(tmpDir, repoRelPath)
	if err := os.MkdirAll(repoFullDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo in the subdirectory x
	runCmd(t, repoFullDir, "git", "init")
	runCmd(t, repoFullDir, "git", "config", "user.name", "Test User")
	runCmd(t, repoFullDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoFullDir, "git", "config", "commit.gpgsign", "false")

	// Create files inside the repo
	mainFilePath := filepath.Join(repoFullDir, "cmd", "main.go")
	if err := os.MkdirAll(filepath.Dir(mainFilePath), 0755); err != nil {
		t.Fatalf("failed to create cmd dir: %v", err)
	}
	initialContent := "package main\n\nfunc main() {}\n"
	if err := ioutil.WriteFile(mainFilePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	// Commit initial content
	runCmd(t, repoFullDir, "git", "add", "cmd/main.go")
	runCmd(t, repoFullDir, "git", "commit", "-m", "initial commit")

	// Modify the file to produce a diff
	modifiedContent := "package main\n\nfunc main() {\n\t// Edited\n}\n"
	if err := ioutil.WriteFile(mainFilePath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to write modified file: %v", err)
	}

	// Add an untracked file to test git add -N . behavior inside GetWorkspaceDiff
	newFilePath := filepath.Join(repoFullDir, "cmd", "helper.go")
	if err := ioutil.WriteFile(newFilePath, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	// Write metadata.json at the workspace root
	meta := map[string]interface{}{
		"repos": []map[string]interface{}{
			{
				"name": "x",
				"paths": map[string]interface{}{
					"main": repoRelPath,
				},
			},
		},
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpDir, "metadata.json"), metaBytes, 0644); err != nil {
		t.Fatalf("failed to write metadata.json: %v", err)
	}

	// Instantiate SandboxGitClient with a local executor
	client := NewSandboxGitClient(
		func(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
			// Execute the command locally using bash -c
			cmd := exec.Command("bash", "-c", command)
			stdout, err := cmd.Output()
			if err != nil {
				var stderr []byte
				if exitErr, ok := err.(*exec.ExitError); ok {
					stderr = exitErr.Stderr
				}
				return map[string]any{
					"stdout": string(stdout),
					"stderr": string(stderr),
				}, err
			}
			return map[string]any{
				"stdout": string(stdout),
			}, nil
		},
		func(ctx context.Context, taskID string, jobID *string, level string, message string) {},
	)

	// Test GetWorkspaceDiff
	diffText, err := client.GetWorkspaceDiff(context.Background(), &models.Task{}, nil, tmpDir, "")
	if err != nil {
		t.Fatalf("GetWorkspaceDiff failed: %v", err)
	}

	// Check if diff output carries repo-relative paths (headers are a/cmd/main.go)
	if !strings.Contains(diffText, "--- Repository: x") {
		t.Errorf("diff does not contain repository attribution '--- Repository: x', got: %q", diffText)
	}
	if !strings.Contains(diffText, "--- a/cmd/main.go") {
		t.Errorf("diff does not contain repo-relative path '--- a/cmd/main.go', got: %q", diffText)
	}
	if !strings.Contains(diffText, "+++ b/cmd/main.go") {
		t.Errorf("diff does not contain repo-relative path '+++ b/cmd/main.go', got: %q", diffText)
	}
	// Verify that it tracked untracked files
	if !strings.Contains(diffText, "+++ b/cmd/helper.go") {
		t.Errorf("diff does not track helper.go: %q", diffText)
	}

	// Test GetWorkspaceChangedFiles
	changedFiles, err := client.GetWorkspaceChangedFiles(context.Background(), &models.Task{}, nil, tmpDir, "")
	if err != nil {
		t.Fatalf("GetWorkspaceChangedFiles failed: %v", err)
	}

	if len(changedFiles) != 2 {
		t.Errorf("expected 2 changed files, got %d: %v", len(changedFiles), changedFiles)
	}
	foundMain := false
	foundHelper := false
	for _, f := range changedFiles {
		if f == "cmd/main.go" {
			foundMain = true
		}
		if f == "cmd/helper.go" {
			foundHelper = true
		}
	}
	if !foundMain {
		t.Errorf("expected 'cmd/main.go' in changed files: %v", changedFiles)
	}
	if !foundHelper {
		t.Errorf("expected 'cmd/helper.go' in changed files: %v", changedFiles)
	}
}
