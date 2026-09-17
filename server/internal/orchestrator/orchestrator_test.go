package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/patch"
	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/wkspace"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestOrchestrator_StepPR_NoChangesMerged(t *testing.T) {
	task := &models.Task{
		ID:          "task-no-change",
		ProjectID:   "proj-no-change",
		Title:       "No change task",
		Description: "Nothing should be committed.",
		Complexity:  models.TaskComplexityMedium,
		Status:      models.TaskStatusTesting,
	}
	job := &models.WorkflowJob{ID: "job-no-change", TaskID: task.ID, Status: models.WorkflowJobStatusQueued}
	agent := &models.Agent{ID: "agent-123", Name: "Test Agent", Role: models.AgentRoleBackend}
	repo := models.Repository{ID: "repo-123", ProjectID: task.ProjectID, URL: "https://github.com/test/repo.git", Branch: "main", Token: "token-123"}

	taskRepo := &mockTaskRepo{task: task}
	workflowRepo := &mockWorkflowRepo{job: job}
	agentAssigner := &mockAgentAssigner{agent: agent}
	sandboxRuntime := &mockSandboxRuntime{}
	gitOps := &mockGitOpsClient{prURLSet: true, prURL: ""}
	artifactRepo := &mockArtifactRepo{}
	reposRepo := &mockRepositoriesRepo{repo: repo}

	orch := New(taskRepo, workflowRepo, agentAssigner, sandboxRuntime,
		WithGitOpsClient(gitOps),
		WithArtifactRepository(artifactRepo),
		WithRepositoryRepository(reposRepo),
	)

	runners := orch.stepRunners(task, agent, job.ID, job.Step)
	runner := runners[workflow.StepPR]
	if runner == nil {
		t.Fatal("missing PR step runner")
	}

	_, err := runner(context.Background(), workflow.StepContext{Inputs: map[string]map[string]any{
		workflow.StepTest: map[string]any{"status": "passed"},
	}})
	if !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Fatalf("expected workflow.ErrWaitingApproval, got: %v", err)
	}
	if task.Status != models.TaskStatusPrReady {
		t.Fatalf("expected task status pr_ready, got %s", task.Status)
	}
	if len(taskRepo.updated) == 0 {
		t.Fatal("expected task updates to be recorded")
	}
	for _, update := range taskRepo.updated {
		if update.PRURLs != nil {
			t.Fatalf("expected no PR URL update when no changes are detected, got %v", []string(*update.PRURLs))
		}
		if update.Status != nil && *update.Status != models.TaskStatusPrReady {
			t.Fatalf("expected only pr_ready status update, got %s", *update.Status)
		}
	}
}

func TestOrchestrator_RemoveWorkspaceSafety(t *testing.T) {
	root := t.TempDir()
	orch := New(nil, nil, nil, nil, WithWorkspaceRoot(root))
	orch.initWkspace()

	if err := os.MkdirAll(filepath.Join(root, "task-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := orch.wkspace.RemoveWorkspace("task-1"); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "task-1")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace removed, got err=%v", err)
	}
	if err := orch.wkspace.RemoveWorkspace(""); err == nil {
		t.Fatal("expected empty task id to be rejected")
	}
	if err := orch.wkspace.RemoveWorkspace("../outside"); err == nil {
		t.Fatal("expected escaping task id to be rejected")
	}
}

func TestOrchestrator_PruneWorkspacesHonorsRetention(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "old-task")
	newDir := filepath.Join(root, "new-task")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	orch := New(nil, nil, nil, nil,
		WithWorkspaceRoot(root),
		WithWorkspaceRetention(time.Hour, time.Hour),
	)
	orch.initWkspace()

	removed, err := orch.wkspace.PruneWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("prune workspaces: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed workspace, got %d", removed)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("expected old workspace removed, got err=%v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("expected new workspace retained: %v", err)
	}
}

func TestOrchestrator_PruneLogsHonorsRetention(t *testing.T) {
	root := t.TempDir()
	oldFile := filepath.Join(root, "old-task.jsonl")
	newFile := filepath.Join(root, "new-task.jsonl")
	if err := os.WriteFile(oldFile, []byte("log line 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("log line 2"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -15) // older than 14 days
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	removed, err := wkspace.PruneLogFiles(context.Background(), 14, root)
	if err != nil {
		t.Fatalf("prune log files: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed log file, got %d", removed)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old log file removed, got err=%v", err)
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("expected new log file retained: %v", err)
	}
}

func TestMatchAffectedFile(t *testing.T) {
	tests := []struct {
		pattern  string
		file     string
		expected bool
	}{
		// Exact Match
		{"internal/app/backend/handlers.go", "internal/app/backend/handlers.go", true},
		{"readme.md", "code/repos/test/main/readme.md", true},
		{"Dockerfile", "code/repos/test/main/Dockerfile", true},
		{"src/readme.md", "code/repos/test/main/src/readme.md", true},
		{"internal/app/backend/handlers.go", "internal/app/backend/other.go", false},

		// Directory Prefix Match
		{"internal/app/backend", "internal/app/backend/handlers.go", true},
		{"internal/app/backend", "internal/app/backend/models/user.go", true},
		{"internal/app/backend", "internal/app/backend-other/handlers.go", false},

		// Glob Match
		{"*.go", "main.go", true},
		{"*.go", "internal/app/backend/handlers.go", false},             // base name match fallback disabled for safety
		{"internal/**/*.go", "internal/app/backend/handlers.go", false}, // filepath.Match doesn't support ** recursive glob

		// Extension / Description Matches (Should now be false/rejected)
		{"New GoLang source files (.go)", "internal/app/backend/handlers.go", false},
		{"New GoLang source files (.go)", "go.mod", false},
		{"Documentation files (if applicable)", "README.md", false},

		// Catch-all (Should now be false/rejected)
		{"All relevant source files of the original project (language unknown)", "internal/app/backend/handlers.go", false},
		{"All relevant source files", "package.json", false},

		// Filename Inclusion (Should now be false/rejected)
		{"New GoLang project configuration files (e.g., go.mod, Dockerfile for Go)", "go.mod", false},
		{"New GoLang project configuration files (e.g., go.mod, Dockerfile for Go)", "Dockerfile", false},
	}

	for i, tc := range tests {
		actual := patch.MatchAffectedFile(tc.pattern, tc.file)
		if actual != tc.expected {
			t.Errorf("Test %d failed: patch.MatchAffectedFile(%q, %q) expected %v, got %v", i, tc.pattern, tc.file, tc.expected, actual)
		}
	}
}

type testMockRepositoriesRepo struct {
	repos []models.Repository
}

func (m *testMockRepositoriesRepo) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return m.repos, nil
}

func (m *testMockRepositoriesRepo) ListAll(ctx context.Context) ([]models.Repository, error) {
	return m.repos, nil
}

func TestOrchestrator_GetTaskRepoHostPath_FailsOnRepositoryIDMismatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repo-path-mismatch-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	task := &models.Task{ID: "task-path", ProjectID: "proj-path", RepositoryID: &repoID}
	orch := &Orchestrator{
		workspaceRoot: tmpDir,
		repositories: &testMockRepositoriesRepo{repos: []models.Repository{{
			ID:  "repo-b",
			URL: "https://github.com/example/repo-b.git",
		}}},
	}
	orch.initWkspace()
	orch.initRepoutil()

	ws := orch.wkspace.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: "repo-b",
		Name:   "repo-b",
		Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-b", "main")},
	}}
	if err := os.MkdirAll(ws.Root, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	if err := orch.wkspace.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	if _, err := orch.repoutil.GetTaskRepoHostPath(context.Background(), task); err == nil {
		t.Fatal("expected repository ID mismatch to fail fast")
	}
}

func TestOrchestrator_ReadAffectedFileContent_ResolvesRepoRelativePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "affected-file-root-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	task := &models.Task{ID: "task-affected", ProjectID: "proj-affected", RepositoryID: &repoID}
	orch := &Orchestrator{workspaceRoot: tmpDir}
	orch.initWkspace()
	ws := orch.wkspace.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: repoID,
		Name:   "repo-a",
		Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-a", "main")},
	}}
	repoRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
	if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := orch.wkspace.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	content, ok := orch.readAffectedFileContent(context.Background(), task, "src/main.go")
	if !ok {
		t.Fatal("expected repo-relative affected file to resolve")
	}
	if !strings.Contains(content, "package main") {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestOrchestrator_ReadAffectedFileContent_WorktreeFirst(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "affected-file-worktree-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	agentID := "agent-123"
	task := &models.Task{
		ID:           "task-affected-wt",
		ProjectID:    "proj-affected-wt",
		RepositoryID: &repoID,
		AgentID:      &agentID,
	}

	agent := &models.Agent{
		ID:   agentID,
		Role: "backend",
	}

	taskRepo := &mockTaskRepo{task: task}
	agentAssigner := &mockAgentAssigner{agent: agent}

	orch := New(taskRepo, nil, agentAssigner, nil, WithWorkspaceRoot(tmpDir))
	orch.initWkspace()
	ws := orch.wkspace.GetTaskWorkspace(task)
	ws.Repos = []models.RepoWorkspace{{
		RepoID: repoID,
		Name:   "repo-a",
		Paths: models.RepoWorkspacePaths{
			Main: filepath.Join("repos", "repo-a", "main"),
			Worktrees: map[string]string{
				"backend": filepath.Join("repos", "repo-a", "worktrees", "backend"),
			},
		},
	}}

	mainRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
	wtRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Worktrees["backend"])

	if err := os.MkdirAll(filepath.Join(mainRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create main repo dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(wtRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create wt repo dir: %v", err)
	}

	// Write different content to main vs worktree
	if err := os.WriteFile(filepath.Join(mainRoot, "src", "main.go"), []byte("package main\n// main\n"), 0o644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtRoot, "src", "main.go"), []byte("package main\n// worktree\n"), 0o644); err != nil {
		t.Fatalf("failed to write wt file: %v", err)
	}
	// Write a file that only exists in worktree
	if err := os.WriteFile(filepath.Join(wtRoot, "src", "wt_only.go"), []byte("package main\n// wt only\n"), 0o644); err != nil {
		t.Fatalf("failed to write wt only file: %v", err)
	}

	if err := orch.wkspace.SaveTaskWorkspaceMetadata(task, ws); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// 1. Verify we read the worktree version for a file in both
	content, ok := orch.readAffectedFileContent(context.Background(), task, "src/main.go")
	if !ok {
		t.Fatal("expected file to resolve")
	}
	if !strings.Contains(content, "// worktree") {
		t.Fatalf("expected worktree content, got %q", content)
	}

	// 2. Verify we read the file that only exists in worktree
	contentOnly, okOnly := orch.readAffectedFileContent(context.Background(), task, "src/wt_only.go")
	if !okOnly {
		t.Fatal("expected wt_only file to resolve")
	}
	if !strings.Contains(contentOnly, "// wt only") {
		t.Fatalf("expected wt only content, got %q", contentOnly)
	}
}

func TestOrchestrator_ClearCheckpointsForRepair(t *testing.T) {
	tests := []struct {
		name     string
		task     *models.Task
		expected []string
	}{
		{
			name: "PrunesDynamicSubsteps_MediumComplexity",
			task: &models.Task{
				ID:         "task-repair-123",
				ProjectID:  "proj-repair-123",
				Complexity: models.TaskComplexityMedium,
			},
			expected: []string{"code_backend", "code_frontend", "review", "fix", "test", "pr"},
		},
		{
			name: "EasyClearsCodingSteps",
			task: &models.Task{
				ID:         "task-repair-456",
				ProjectID:  "proj-repair-456",
				Complexity: models.TaskComplexityEasy,
			},
			expected: []string{"code_backend", "code_frontend", "review", "fix", "test", "pr"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taskRepo := &mockTaskRepo{task: tc.task}
			workflowRepo := &mockWorkflowRepo{}
			orch := New(taskRepo, workflowRepo, nil, nil)

			if err := orch.ClearCheckpointsForRepair(context.Background(), tc.task.ID); err != nil {
				t.Fatalf("ClearCheckpointsForRepair failed: %v", err)
			}

			if len(workflowRepo.deletedSteps) != len(tc.expected) {
				t.Fatalf("unexpected deleted step count: got %v want %v", workflowRepo.deletedSteps, tc.expected)
			}
			for i, step := range tc.expected {
				if workflowRepo.deletedSteps[i] != step {
					t.Fatalf("unexpected deleted step at %d: got %q want %q", i, workflowRepo.deletedSteps[i], step)
				}
			}
		})
	}
}

func TestOrchestrator_WorktreeAndPatchHelpers(t *testing.T) {
	// 1. Test splitPatchByRepo
	patchText := `Some header info
diff --git a/repo1/src/main.go b/repo1/src/main.go
index 123456..789012 100644
--- a/repo1/src/main.go
+++ b/repo1/src/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
diff --git a/repo2/index.js b/repo2/index.js
index abc..def 100644
--- a/repo2/index.js
+++ b/repo2/index.js
@@ -1,1 +1,2 @@
 console.log("hello");
+console.log("world");
`

	splitPatches := patch.SplitPatchByRepo(patchText)
	if len(splitPatches) != 2 {
		t.Errorf("expected 2 split patches, got: %d", len(splitPatches))
	}
	if !strings.Contains(splitPatches["repo1"], "src/main.go") {
		t.Errorf("expected repo1 patch to contain main.go, got: %s", splitPatches["repo1"])
	}
	if strings.Contains(splitPatches["repo1"], "index.js") {
		t.Errorf("repo1 patch should not contain repo2 changes")
	}
	if !strings.Contains(splitPatches["repo2"], "index.js") {
		t.Errorf("expected repo2 patch to contain index.js, got: %s", splitPatches["repo2"])
	}

	// 2. Test hostWorktreePath and containerPathForHostPath
	orch := &Orchestrator{
		workspaceRoot: "/tmp/workspaces",
	}
	orch.initRepoutil()

	// Single repo case
	repoID := "single-repo-id"
	singleTask := &models.Task{
		ID:           "task-1",
		RepositoryID: &repoID,
	}

	localPath := "/tmp/workspaces/task-1"
	repo := models.Repository{
		ID:  repoID,
		URL: "https://github.com/example/repo-a.git",
	}

	repoPathWithoutMetadata := orch.repoutil.RepoHostPath(singleTask, nil, repo)
	expectedRepoPathWithoutMetadata := filepath.Clean("/tmp/workspaces/task-1/code/repos/repo-a/main")
	if filepath.Clean(repoPathWithoutMetadata) != expectedRepoPathWithoutMetadata {
		t.Errorf("expected repo path without metadata: %s, got: %s", expectedRepoPathWithoutMetadata, repoPathWithoutMetadata)
	}

	// Suffix case
	bePath := orch.repoutil.HostWorktreePath(singleTask, localPath, "-be-worktree")
	expectedBePath := filepath.Clean("/tmp/workspaces/task-1/be")
	if filepath.Clean(bePath) != expectedBePath {
		t.Errorf("expected single-repo bePath: %s, got: %s", expectedBePath, bePath)
	}

	containerBe := orch.containerPathForHostPath(singleTask, bePath, "-be-worktree")
	if containerBe != "/workspace/be" {
		t.Errorf("expected containerPath for worktree to be /workspace/be, got: %s", containerBe)
	}

	// Child in worktree case
	containerBeFile := orch.containerPathForHostPath(singleTask, filepath.Join(bePath, "src/main.go"), "-be-worktree")
	if containerBeFile != "/workspace/be/src/main.go" {
		t.Errorf("expected container path /workspace/be/src/main.go, got: %s", containerBeFile)
	}

	// Multi repo case
	multiTask := &models.Task{
		ID:           "task-2",
		RepositoryID: nil,
	}

	multiRepoPathWithoutMetadata := orch.repoutil.RepoHostPath(multiTask, nil, repo)
	expectedMultiRepoPathWithoutMetadata := filepath.Clean("/tmp/workspaces/task-2/code/repos/repo-a/main")
	if filepath.Clean(multiRepoPathWithoutMetadata) != expectedMultiRepoPathWithoutMetadata {
		t.Errorf("expected multi repo path without metadata: %s, got: %s", expectedMultiRepoPathWithoutMetadata, multiRepoPathWithoutMetadata)
	}

	multiLocalPath := "/tmp/workspaces/task-2"
	multiRepoPath := filepath.Join(multiLocalPath, "repo-a")

	multiBePath := orch.repoutil.HostWorktreePath(multiTask, multiRepoPath, "-be-worktree")
	expectedMultiBePath := filepath.Clean("/tmp/workspaces/task-2/repo-a-be-worktree")
	if filepath.Clean(multiBePath) != expectedMultiBePath {
		t.Errorf("expected multi-repo bePath: %s, got: %s", expectedMultiBePath, multiBePath)
	}

	multiContainerBe := orch.containerPathForHostPath(multiTask, multiBePath, "-be-worktree")
	// Since active step workspace root is multiLocalPath (mounted to /workspace):
	// multiBePath relative to multiLocalPath is "repo-a-be-worktree"
	if multiContainerBe != "/workspace/repo-a-be-worktree" {
		t.Errorf("expected multi-repo container path: /workspace/repo-a-be-worktree, got: %s", multiContainerBe)
	}
}

func TestTestProjectDetectionAndTargetedCommands(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-project-detection-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cases := []struct {
		name       string
		marker     string
		changed    string
		wantKind   orchtester.ProjectKind
		wantInCmd  string
		goPackages map[string]bool
	}{
		{
			name:       "go",
			marker:     "go.mod",
			changed:    "pkg/service.go",
			wantKind:   orchtester.ProjectGo,
			wantInCmd:  "go test -v ./pkg/...",
			goPackages: map[string]bool{"./pkg": true},
		},
		{name: "node", marker: "package.json", changed: "src/app.ts", wantKind: orchtester.ProjectJS, wantInCmd: "npm test"},
		{name: "python", marker: "pyproject.toml", changed: "app/main.py", wantKind: orchtester.ProjectPython, wantInCmd: "pytest"},
		{name: "java", marker: "pom.xml", changed: "src/App.java", wantKind: orchtester.ProjectJava, wantInCmd: "mvn test"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(tmpDir, tc.name)
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(tc.changed)), 0o755); err != nil {
				t.Fatalf("failed to create source dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, tc.marker), []byte("marker"), 0o644); err != nil {
				t.Fatalf("failed to write marker: %v", err)
			}

			kind, markers := orchtester.DetectProjectKindNear(root, tc.changed)
			if kind != tc.wantKind {
				t.Fatalf("expected kind %s, got %s", tc.wantKind, kind)
			}
			if len(markers) == 0 {
				t.Fatal("expected markers for detected project kind")
			}

			goPackages := tc.goPackages
			if goPackages == nil {
				goPackages = map[string]bool{}
			}
			cmd, ok := orchtester.TargetedTestCommand(kind, "/workspace", []string{tc.changed}, goPackages)
			if !ok {
				t.Fatal("expected targeted command")
			}
			if !strings.Contains(cmd, tc.wantInCmd) {
				t.Fatalf("expected command to contain %q, got %q", tc.wantInCmd, cmd)
			}
		})
	}
}

func TestFullVerificationScriptUsesSharedMarkers(t *testing.T) {
	script := orchtester.FullVerificationScript()
	for _, expected := range []string{"go.mod", "package.json", "requirements.txt", "pyproject.toml", "pytest.ini", "pom.xml", "build.gradle"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("expected full verification script to contain marker %s", expected)
		}
	}
}
