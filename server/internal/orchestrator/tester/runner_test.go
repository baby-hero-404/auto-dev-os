package tester

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestRunner_RunTargetedTests_PathParsing(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create repo-a main worktree with go.mod
	repoABePath := filepath.Join(tmpDir, "code/repos/repo-a/main-be")
	if err := os.MkdirAll(repoABePath, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoABePath, "go.mod"), []byte("module repo-a"), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create repo-b main worktree with package.json
	repoBFePath := filepath.Join(tmpDir, "code/repos/repo-b/main-fe")
	if err := os.MkdirAll(repoBFePath, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoBFePath, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	var resolvedAbsModDirs []string

	runner := Runner{
		ResolveRepoHostPath: func(ctx context.Context, task *models.Task) (string, error) {
			return tmpDir, nil
		},
		HostWorktreePath: func(task *models.Task, base string, suffix string) string {
			return base + suffix
		},
		ContainerPathForHostPath: func(task *models.Task, host string, suffix string) string {
			resolvedAbsModDirs = append(resolvedAbsModDirs, host)
			return "/workspace"
		},
		RunSandboxStepInWorktree: func(ctx context.Context, task *models.Task, agent *models.Agent, stepName, cmd, suffix string) (map[string]any, error) {
			return map[string]any{"status": "success"}, nil
		},
		SaveArtifact: func(ctx context.Context, jobID, taskID, step, name string, data any) error {
			return nil
		},
		Log: func(ctx context.Context, taskID string, jobID *string, level, msg string) {},
	}

	task := &models.Task{ID: "task-123"}
	agent := &models.Agent{ID: "agent-123"}

	// 1. Multi-repo path under main
	_, err := runner.RunTargetedTests(context.Background(), task, agent, "job-1", "test", []string{"code/repos/repo-a/main/src/main.go"}, "-be")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolvedAbsModDirs) != 1 || resolvedAbsModDirs[0] != repoABePath {
		t.Errorf("expected resolved path %q, got %v", repoABePath, resolvedAbsModDirs)
	}

	resolvedAbsModDirs = nil

	// 2. Multi-repo path under worktree
	_, err = runner.RunTargetedTests(context.Background(), task, agent, "job-1", "test", []string{"code/repos/repo-b/worktrees/be/src/app.ts"}, "-fe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolvedAbsModDirs) != 1 || resolvedAbsModDirs[0] != repoBFePath {
		t.Errorf("expected resolved path %q, got %v", repoBFePath, resolvedAbsModDirs)
	}
}
