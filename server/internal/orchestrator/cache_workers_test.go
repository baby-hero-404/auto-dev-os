package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockContextEngine struct {
	indexedRoots []string
}

func (m *mockContextEngine) GetRepoMap(ctx context.Context, activeFiles []string, maxTokens int) (string, error) {
	return "", nil
}

func (m *mockContextEngine) RetrieveContext(ctx context.Context, taskQuery string, limit int) ([]models.ContextSnippet, error) {
	return nil, nil
}

func (m *mockContextEngine) IndexWorkspace(ctx context.Context) error {
	if wsRoot, ok := ctx.Value(provider.WorkspaceRootKey).(string); ok {
		m.indexedRoots = append(m.indexedRoots, wsRoot)
		dbDir := filepath.Join(wsRoot, "context")
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return err
		}
		dbPath := filepath.Join(dbDir, "workspace_cache.db")
		f, err := os.Create(dbPath)
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func (m *mockContextEngine) Close() error {
	return nil
}

func (m *mockContextEngine) GetGlobalCacheDir() string {
	return ""
}

func (m *mockContextEngine) BuildGlobalCache(repoAbsPath string, repoName string, commitHash string) error {
	return nil
}

func (m *mockContextEngine) InitLocalCache(wsRoot string, repoCommits []provider.RepoCommitInfo) error {
	return nil
}

type mockGitOpsForPrewarm struct {
	mockGitOpsClient
}

func (m *mockGitOpsForPrewarm) CloneForTask(ctx context.Context, repoURL, branch, localPath string) (string, error) {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = localPath
	if err := cmd.Run(); err != nil {
		return "", err
	}
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = localPath
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "test")
	cmd.Dir = localPath
	_ = cmd.Run()

	err := os.WriteFile(filepath.Join(localPath, "main.go"), []byte("package main"), 0644)
	if err != nil {
		return "", err
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = localPath
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = localPath
	_ = cmd.Run()

	return branch, nil
}

func TestCacheWorkers_PrewarmAndGC(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-worker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workspaceRoot := filepath.Join(tmpDir, "workspaces")
	dataRoot := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}
	if err := os.MkdirAll(dataRoot, 0755); err != nil {
		t.Fatalf("failed to create data root: %v", err)
	}

	repo := models.Repository{
		ID:        "repo-123",
		ProjectID: "proj-123",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
	}

	taskRepo := &mockTaskRepo{}
	workflowRepo := &mockWorkflowRepo{}
	agentAssigner := &mockAgentAssigner{}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsForPrewarm{}
	reposRepo := &mockRepositoriesRepo{repo: repo}
	ctxEngine := &mockContextEngine{}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithGitOpsClient(gitOps),
		WithRepositoryRepository(reposRepo),
		WithWorkspaceRoot(workspaceRoot),
		WithDataRoot(dataRoot),
		WithContextEngine(ctxEngine),
	)

	ctx := context.Background()

	// 1. Run Prewarming
	orch.prewarmAllCaches(ctx)

	// Check that global cache database file was created
	globalCacheDir := filepath.Join(dataRoot, "database", "global_cache")
	entries, err := os.ReadDir(globalCacheDir)
	if err != nil {
		t.Fatalf("failed to read global cache dir: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 global cache file, got %d", len(entries))
	}

	createdFileName := entries[0].Name()
	if !strings.HasPrefix(createdFileName, "global_cache_repo_") {
		t.Errorf("expected file name prefix global_cache_repo_, got %s", createdFileName)
	}

	commitHash := strings.TrimPrefix(createdFileName, "global_cache_repo_")
	commitHash = strings.TrimSuffix(commitHash, ".db")

	// 2. Set up GC scenario with multiple historical cache files
	// commit1: 10 days old (should be cleaned up unless referenced)
	// commit2: 5 days old (should be kept because < 7 days old)
	// commit3: latest (modTime is now, should be kept)
	// commit4: 10 days old, but referenced by active task (should be kept)

	now := time.Now()
	tenDaysAgo := now.AddDate(0, 0, -10)
	fiveDaysAgo := now.AddDate(0, 0, -5)

	_ = os.Remove(filepath.Join(globalCacheDir, createdFileName)) // Clear prewarmed cache to set up mock files

	filesToMock := []struct {
		name    string
		modTime time.Time
	}{
		{"global_cache_repo_commit1.db", tenDaysAgo},
		{"global_cache_repo_commit2.db", fiveDaysAgo},
		{"global_cache_repo_commit3.db", now},
		{"global_cache_repo_commit4.db", tenDaysAgo},
	}

	for _, f := range filesToMock {
		fPath := filepath.Join(globalCacheDir, f.name)
		if err := os.WriteFile(fPath, []byte("sqlite"), 0644); err != nil {
			t.Fatalf("failed to write mock db %s: %v", f.name, err)
		}
		if err := os.Chtimes(fPath, f.modTime, f.modTime); err != nil {
			t.Fatalf("failed to set times for %s: %v", f.name, err)
		}
	}

	// Create referenced workspace on disk for commit4
	referencedWkspaceDir := filepath.Join(workspaceRoot, "task-active")
	repoMainPath := filepath.Join(referencedWkspaceDir, "code/repos/repo/main")
	if err := os.MkdirAll(repoMainPath, 0755); err != nil {
		t.Fatalf("failed to create repo main path: %v", err)
	}

	// Initialize git repo in repoMainPath with commit4
	cmd := exec.Command("git", "init")
	cmd.Dir = repoMainPath
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = localPathConf(repoMainPath)
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "test")
	cmd.Dir = localPathConf(repoMainPath)
	_ = cmd.Run()
	_ = os.WriteFile(filepath.Join(repoMainPath, "dummy"), []byte("d"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoMainPath
	_ = cmd.Run()
	// commit4 has modTime of 10 days ago but git commit will run now
	cmd = exec.Command("git", "commit", "-m", "commit4")
	cmd.Dir = repoMainPath
	_ = cmd.Run()

	commit4Hash, errCommit := runGitCmd(repoMainPath, "rev-parse", "HEAD")
	if errCommit != nil {
		t.Fatalf("failed to get commit4 hash: %v", errCommit)
	}

	// Rename mock commit4 file to match actual git commit4 hash
	oldCommit4Path := filepath.Join(globalCacheDir, "global_cache_repo_commit4.db")
	actualCommit4Path := filepath.Join(globalCacheDir, fmt.Sprintf("global_cache_repo_%s.db", commit4Hash))
	_ = os.Rename(oldCommit4Path, actualCommit4Path)

	// Write metadata.json for the workspace
	type metaRepo struct {
		Name  string `json:"name"`
		Paths struct {
			Main string `json:"main"`
		} `json:"paths"`
	}
	type taskMeta struct {
		Repos []metaRepo `json:"repos"`
	}
	meta := taskMeta{
		Repos: []metaRepo{
			{
				Name: "repo",
				Paths: struct {
					Main string `json:"main"`
				}{
					Main: "code/repos/repo/main",
				},
			},
		},
	}
	metaBytes, _ := json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(referencedWkspaceDir, "metadata.json"), metaBytes, 0644)

	// 3. Run Garbage Collection
	orch.runGarbageCollection(ctx)

	// Verify database files after GC
	remainingFiles, err := os.ReadDir(globalCacheDir)
	if err != nil {
		t.Fatalf("failed to read global cache dir after GC: %v", err)
	}

	remainingNames := make(map[string]bool)
	for _, entry := range remainingFiles {
		remainingNames[entry.Name()] = true
	}

	// Expected results:
	// - commit1.db: Deleted (older than 7 days, not referenced)
	// - commit2.db: Kept (only 5 days old)
	// - commit3.db: Kept (newest version)
	// - commit4 (actualCommit4Path): Kept (referenced by active task)

	if remainingNames["global_cache_repo_commit1.db"] {
		t.Errorf("expected global_cache_repo_commit1.db to be deleted")
	}
	if !remainingNames["global_cache_repo_commit2.db"] {
		t.Errorf("expected global_cache_repo_commit2.db to be kept")
	}
	if !remainingNames["global_cache_repo_commit3.db"] {
		t.Errorf("expected global_cache_repo_commit3.db to be kept")
	}
	if !remainingNames[filepath.Base(actualCommit4Path)] {
		t.Errorf("expected referenced cache %s to be kept", filepath.Base(actualCommit4Path))
	}
}

func localPathConf(p string) string {
	return p
}
