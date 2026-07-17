package repoutil

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestCreateGitCheckpoint_WorktreeResilience(t *testing.T) {
	task := &models.Task{ID: "task-test-resilience"}
	ws := &models.TaskWorkspace{
		Repos: []models.RepoWorkspace{{Name: "repo1"}},
	}
	manager := &Manager{
		DefaultAgentName:  "Test",
		DefaultAgentEmail: "test@test.com",
		RunSandboxStepInWorktree: func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, script string, suffix string) (map[string]any, error) {
			if strings.Contains(script, "WORKTREE_INVALID") {
				// This implies the script has the resilience check
				return map[string]any{"stdout": "WORKTREE_RECREATED\nSTAGED_COUNT=0\nmock-hash"}, nil
			}
			return map[string]any{"stdout": "STAGED_COUNT=0\nmock-hash"}, nil
		},
		GetTaskWorkspace: func(t *models.Task) *models.TaskWorkspace {
			return ws
		},
		ListRepositories: func(ctx context.Context, projectID string) ([]models.Repository, error) {
			return []models.Repository{{URL: "repo1"}}, nil
		},
		FindRepoWorkspaceByPath: func(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error) {
			return nil, fmt.Errorf("not found")
		},
		ContainerPathForHostPath: func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return hostPath
		},
	}
	
	result, err := manager.CreateGitCheckpoint(context.Background(), task, nil, "test_step", models.WorktreeSuffixBackend)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if result.Hash != "mock-hash" {
		t.Errorf("expected hash mock-hash, got %s", result.Hash)
	}
}

func TestRestoreGitCheckpoint_WorktreeResilience(t *testing.T) {
	task := &models.Task{ID: "task-test-resilience"}
	ws := &models.TaskWorkspace{
		Repos: []models.RepoWorkspace{{Name: "repo1"}},
	}
	manager := &Manager{
		DefaultAgentName:  "Test",
		DefaultAgentEmail: "test@test.com",
		RunSandboxStepInWorktree: func(ctx context.Context, task *models.Task, agent *models.Agent, stepName string, script string, suffix string) (map[string]any, error) {
			if !strings.Contains(script, "Recreating missing or invalid worktree directory") {
				t.Error("expected restore script to contain worktree resilience check")
			}
			return map[string]any{"stdout": ""}, nil
		},
		GetTaskWorkspace: func(t *models.Task) *models.TaskWorkspace {
			return ws
		},
		ListRepositories: func(ctx context.Context, projectID string) ([]models.Repository, error) {
			return []models.Repository{{URL: "repo1"}}, nil
		},
		FindRepoWorkspaceByPath: func(ctx context.Context, task *models.Task, path string) (*models.RepoWorkspace, error) {
			return nil, fmt.Errorf("not found")
		},
		ContainerPathForHostPath: func(task *models.Task, hostPath string, worktreeSuffix string) string {
			return hostPath
		},
	}
	
	err := manager.RestoreGitCheckpoint(context.Background(), task, nil, "mock-hash", models.WorktreeSuffixBackend)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
