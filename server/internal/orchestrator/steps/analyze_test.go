package steps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type mockAnalyzeSandboxRuntime struct {
	commands []string
	outputs  map[string]string
}

func (m *mockAnalyzeSandboxRuntime) Run(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
	cmd := ""
	if len(req.Command) >= 3 {
		cmd = req.Command[2]
	}
	m.commands = append(m.commands, cmd)
	for contains, out := range m.outputs {
		if strings.Contains(cmd, contains) {
			return &sandbox.CommandResult{ExitCode: 0, Stdout: out}, nil
		}
	}
	return &sandbox.CommandResult{ExitCode: 0, Stdout: ""}, nil
}

func (m *mockAnalyzeSandboxRuntime) Health(ctx context.Context) error {
	return nil
}

func (m *mockAnalyzeSandboxRuntime) Prewarm(ctx context.Context) error {
	return nil
}

type mockWorkspaceLoader struct {
	ws  *models.TaskWorkspace
	err error
}

func (m *mockWorkspaceLoader) LoadTaskWorkspace(ctx context.Context, task *models.Task) (*models.TaskWorkspace, error) {
	return m.ws, m.err
}

func (m *mockWorkspaceLoader) SaveTaskWorkspaceMetadata(task *models.Task, ws *models.TaskWorkspace) error {
	return nil
}

func TestOrchestrator_AnalyzeToolsUseSourceRootAndExcludeGeneratedDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-source-root-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoID := "repo-a"
	task := &models.Task{ID: "task-analyze-root", ProjectID: "proj-analyze-root", RepositoryID: &repoID}
	analyzeRuntime := &mockAnalyzeSandboxRuntime{outputs: map[string]string{
		"find .":    "src/main.go\n",
		"grep -RIn": "./src/main.go:2: const marker = true\n",
	}}

	ws := &models.TaskWorkspace{
		Root: filepath.Join(tmpDir, task.ID),
		Repos: []models.RepoWorkspace{{
			RepoID: repoID,
			Name:   "repo-a",
			Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-a", "main")},
		}},
	}
	repoRoot := filepath.Join(ws.Root, ws.Repos[0].Paths.Main)
	if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
		t.Fatalf("failed to create repo src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "main.go"), []byte("package main\nconst marker = true\n"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws.Root, "logs"), 0o755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws.Root, "logs", "llm.txt"), []byte("marker in generated logs\n"), 0o644); err != nil {
		t.Fatalf("failed to write generated file: %v", err)
	}

	agent := &models.Agent{ID: "agent-analyze"}
	containerPathFn := func(task *models.Task, hostPath string, worktreeSuffix string) string {
		localPath := sandbox.WorkspacePath(tmpDir, task.ID)
		return "/workspace" + strings.TrimPrefix(hostPath, localPath)
	}

	step := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: agent},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
		nil,
		nil,
		nil,
		sandboxRunnerAdapter{run: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error) {
			res, err := analyzeRuntime.Run(ctx, sandbox.CommandRequest{
				Command: []string{"bash", "-lc", command},
			})
			if err != nil {
				return nil, err
			}
			return StepResult{"exit_code": res.ExitCode, "stdout": res.Stdout, "stderr": res.Stderr}, nil
		}},
		nil,
		nil,
		nil,
		&mockLogger{},
		&mockWorkspaceLoader{ws: ws},
		containerPathFn,
	)

	files, err := step.listAnalyzeFiles(context.Background())
	if err != nil {
		t.Fatalf("listAnalyzeFiles returned error: %v", err)
	}
	if !strings.Contains(files, "src/main.go") {
		t.Fatalf("expected source file in analyze list, got: %s", files)
	}
	if strings.Contains(files, "logs/llm.txt") || strings.Contains(files, "repos") {
		t.Fatalf("analyze list should not expose generated workspace paths, got: %s", files)
	}

	grepResult, err := step.grepAnalyzeFiles(context.Background(), "marker")
	if err != nil {
		t.Fatalf("grepAnalyzeFiles returned error: %v", err)
	}
	if !strings.Contains(grepResult, "src/main.go") {
		t.Fatalf("expected grep to find source file, got: %s", grepResult)
	}
	if strings.Contains(grepResult, "logs/llm.txt") {
		t.Fatalf("grep should not search generated logs, got: %s", grepResult)
	}
	if len(analyzeRuntime.commands) == 0 || !strings.Contains(strings.Join(analyzeRuntime.commands, "\n"), "find .") {
		t.Fatalf("expected analyze tools to run through sandbox commands, got: %#v", analyzeRuntime.commands)
	}
}

func TestOrchestrator_AnalyzeToolsPrefixMultiRepoPaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-multi-repo-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	task := &models.Task{ID: "task-analyze-multi", ProjectID: "proj-analyze-multi"}
	analyzeRuntime := &mockAnalyzeSandboxRuntime{outputs: map[string]string{
		"find .":        "src/main.go\n",
		"head -c 20000": "repo-b marker\n",
	}}

	ws := &models.TaskWorkspace{
		Root: filepath.Join(tmpDir, task.ID),
		Repos: []models.RepoWorkspace{
			{
				RepoID: "repo-a",
				Name:   "repo-a",
				Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-a", "main")},
			},
			{
				RepoID: "repo-b",
				Name:   "repo-b",
				Paths:  models.RepoWorkspacePaths{Main: filepath.Join("repos", "repo-b", "main")},
			},
		},
	}
	for _, repo := range ws.Repos {
		repoRoot := filepath.Join(ws.Root, repo.Paths.Main)
		if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
			t.Fatalf("failed to create repo src: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "src", "main.go"), []byte(repo.Name+" marker\n"), 0o644); err != nil {
			t.Fatalf("failed to write source file: %v", err)
		}
	}

	agent := &models.Agent{ID: "agent-analyze"}
	containerPathFn := func(task *models.Task, hostPath string, worktreeSuffix string) string {
		localPath := sandbox.WorkspacePath(tmpDir, task.ID)
		return "/workspace" + strings.TrimPrefix(hostPath, localPath)
	}

	step := NewAnalyzeStep(
		StepRuntime{Task: task, Agent: agent},
		tmpDir,
		&mockTaskReader{task: task},
		nil,
		nil,
		nil,
		nil,
		sandboxRunnerAdapter{run: func(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, command string) (map[string]any, error) {
			res, err := analyzeRuntime.Run(ctx, sandbox.CommandRequest{
				Command: []string{"bash", "-lc", command},
			})
			if err != nil {
				return nil, err
			}
			return StepResult{"exit_code": res.ExitCode, "stdout": res.Stdout, "stderr": res.Stderr}, nil
		}},
		nil,
		nil,
		nil,
		&mockLogger{},
		&mockWorkspaceLoader{ws: ws},
		containerPathFn,
	)

	files, err := step.listAnalyzeFiles(context.Background())
	if err != nil {
		t.Fatalf("listAnalyzeFiles returned error: %v", err)
	}
	if !strings.Contains(files, "repo-a/src/main.go") || !strings.Contains(files, "repo-b/src/main.go") {
		t.Fatalf("expected prefixed multi-repo files, got: %s", files)
	}

	content, err := step.readAnalyzeFile(context.Background(), "repo-b/src/main.go")
	if err != nil {
		t.Fatalf("readAnalyzeFile returned error: %v", err)
	}
	if !strings.Contains(content, "repo-b marker") {
		t.Fatalf("unexpected repo-b content: %s", content)
	}
}
