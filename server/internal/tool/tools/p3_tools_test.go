package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
)

func TestFindSymbolTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fs-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	content := `package main

const AppVersion = "1.0.0"
var ConfigFile string

type Server struct {
	Port int
}

type Worker interface {
	Start()
}

func GetVersion() string {
	return AppVersion
}

func (s *Server) Listen() {}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	fst := &FindSymbolTool{}

	// Query "GetVersion"
	res, err := fst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query": "GetVersion",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expected success: %s", res.Message)
	}

	var results []SymbolResult
	if err := json.Unmarshal([]byte(res.Output), &results); err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 || results[0].Name != "GetVersion" || results[0].Kind != "function" {
		t.Fatalf("unexpected results: %+v", results)
	}

	// Query "Server"
	res, err = fst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query": "Server",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	var serverResults []SymbolResult
	_ = json.Unmarshal([]byte(res.Output), &serverResults)
	if len(serverResults) != 1 || serverResults[0].Name != "Server" || serverResults[0].Kind != "struct" {
		t.Fatalf("expected 1 match for Server struct, got %d: %+v", len(serverResults), serverResults)
	}

	// Query "Listen"
	res, err = fst.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"query": "Listen",
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	var methodResults []SymbolResult
	_ = json.Unmarshal([]byte(res.Output), &methodResults)
	if len(methodResults) != 1 || methodResults[0].Name != "Listen" || methodResults[0].Kind != "method" {
		t.Fatalf("expected 1 match for Listen method, got %d: %+v", len(methodResults), methodResults)
	}
}

func TestRunLintTool(t *testing.T) {
	mockOut := `{
		"Issues": [
			{
				"FromLinter": "govet",
				"Text": "printf format mismatch",
				"Pos": {
					"Filename": "main.go",
					"Line": 10,
					"Column": 5
				}
			}
		]
	}`

	runtime := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			if !strings.Contains(req.Command[len(req.Command)-1], "golangci-lint run --out-format=json") {
				t.Fatalf("unexpected command: %v", req.Command)
			}
			return &sandbox.CommandResult{
				ExitCode: 1,
				Stdout:   mockOut,
				Stderr:   "",
			}, nil
		},
	}

	rlt := NewRunLintTool(runtime)
	res, err := rlt.Execute(context.Background(), tool.Call{
		Input:     map[string]any{},
		Workspace: "/tmp",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(res.Diagnostics))
	}
	diag := res.Diagnostics[0]
	if diag.File != "main.go" || diag.Line != 10 || !strings.Contains(diag.Message, "printf format mismatch") {
		t.Fatalf("unexpected diagnostic: %+v", diag)
	}
}

func TestGitCheckpointAndRestore(t *testing.T) {
	commandsCalled := []string{}
	runtime := &mockRuntime{
		run: func(ctx context.Context, req sandbox.CommandRequest) (*sandbox.CommandResult, error) {
			cmdStr := strings.Join(req.Command, " ")
			commandsCalled = append(commandsCalled, cmdStr)
			if strings.Contains(cmdStr, "git status") {
				return &sandbox.CommandResult{Stdout: "M main.go\n"}, nil
			}
			if strings.Contains(cmdStr, "git rev-parse HEAD") {
				return &sandbox.CommandResult{Stdout: "abc123commit\n"}, nil
			}
			return &sandbox.CommandResult{ExitCode: 0}, nil
		},
	}

	checkpointTool := NewGitCheckpointTool(runtime)
	resCheck, err := checkpointTool.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"message": "test checkpoint",
		},
		Workspace: "/tmp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resCheck.Success || resCheck.Metadata["commit_hash"] != "abc123commit" {
		t.Fatalf("checkpoint failed: %+v", resCheck)
	}

	restoreTool := NewGitRestoreTool(runtime)
	resRest, err := restoreTool.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"commit_hash": "abc123commit",
		},
		Workspace: "/tmp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resRest.Success {
		t.Fatalf("restore failed: %+v", resRest)
	}

	expectedCmds := []string{
		"git status --porcelain",
		"git add -A && git commit",
		"git rev-parse HEAD",
		"git reset --hard abc123commit",
		"git clean -fd",
	}
	for _, expected := range expectedCmds {
		found := false
		for _, called := range commandsCalled {
			if strings.Contains(called, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected command containing %q to be called, commands run: %v", expected, commandsCalled)
		}
	}
}

func TestCreateFileTool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cf-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "new_file.txt")
	cft := &CreateFileTool{}

	// Test 1: Write file successfully
	res, err := cft.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "new_file.txt",
			"content": "hello world",
			"verify":  false,
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("failed to create file: %s", res.Message)
	}

	data, err := os.ReadFile(filePath)
	if err != nil || string(data) != "hello world" {
		t.Fatalf("unexpected content: %s (err: %v)", string(data), err)
	}

	// Test 2: Refuse to overwrite non-empty file
	resRefuse, err := cft.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "new_file.txt",
			"content": "new content",
			"verify":  false,
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resRefuse.Success {
		t.Fatal("expected overwrite block")
	}

	// Test 3: Verification failure rollback
	mockHook := &mockVerifyHook{
		diags: []tool.Diagnostic{{Severity: "error", Message: "verify failed"}},
	}
	cftWithVerify := &CreateFileTool{
		Verify: &tool.VerifyPipeline{Hooks: []tool.VerifyHook{mockHook}},
	}

	resRollback, err := cftWithVerify.Execute(context.Background(), tool.Call{
		Input: map[string]any{
			"path":    "another_file.txt",
			"content": "unverified",
			"verify":  true,
		},
		Workspace: tmpDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resRollback.Success {
		t.Fatal("expected verification failure rollback")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "another_file.txt")); !os.IsNotExist(err) {
		t.Fatal("expected file to be rolled back and not exist")
	}
}
