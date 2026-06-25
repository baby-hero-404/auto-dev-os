package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillExecutor_ApplyPatchSearchReplace(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(target, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	executor := NewSkillExecutor(nil)
	result := executor.Execute(context.Background(), SkillCall{
		Name:      "apply_patch",
		Workspace: workspace,
		Input: map[string]any{
			"path":    "main.go",
			"search":  "func main() {}",
			"replace": "func main() { println(\"ok\") }",
		},
	})
	if !result.Success {
		t.Fatalf("expected success, got error %q", result.Error)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(string(data), `println("ok")`) {
		t.Fatalf("expected replacement, got:\n%s", string(data))
	}
}

func TestSkillExecutor_RejectsWorkspaceEscape(t *testing.T) {
	executor := NewSkillExecutor(nil)
	result := executor.Execute(context.Background(), SkillCall{
		Name:      "apply_patch",
		Workspace: t.TempDir(),
		Input: map[string]any{
			"path":    "../outside.txt",
			"search":  "a",
			"replace": "b",
		},
	})
	if result.Success {
		t.Fatal("expected workspace escape to fail")
	}
}

func TestSkillExecutor_RejectsUnauthorizedTool(t *testing.T) {
	executor := NewSkillExecutor(nil)
	result := executor.Execute(context.Background(), SkillCall{
		Name:         "apply_patch",
		AgentID:      "agent-1",
		AgentName:    "Backend Agent",
		Workspace:    t.TempDir(),
		AllowedTools: []string{"read_file"},
		Input: map[string]any{
			"path":    "main.go",
			"search":  "old",
			"replace": "new",
		},
	})
	if result.Success {
		t.Fatal("expected unauthorized tool call to fail")
	}
	if result.Error != "agent Backend Agent is not authorized to use tool apply_patch" {
		t.Fatalf("unexpected error: %q", result.Error)
	}
}
