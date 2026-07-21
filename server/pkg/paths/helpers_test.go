package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-safe-path")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	root := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(root, 0755)

	goodFile := filepath.Join(root, "good.txt")
	_ = os.WriteFile(goodFile, []byte("hello"), 0644)

	evilFile := filepath.Join(tmpDir, "task-evil.txt")
	_ = os.WriteFile(evilFile, []byte("secret"), 0644)

	symlinkPath := filepath.Join(root, "bad_link.txt")
	_ = os.Symlink(evilFile, symlinkPath)

	tests := []struct {
		subPath string
		wantErr bool
	}{
		{"good.txt", false},
		{"./good.txt", false},
		{"../task/good.txt", false},
		{"../task-evil.txt", true},
		{"bad_link.txt", true},
		{"../task-evil/somefile", true},
	}

	for _, tc := range tests {
		_, err := ResolveSafePath(root, tc.subPath)
		if (err != nil) != tc.wantErr {
			t.Errorf("ResolveSafePath(%q, %q) error = %v, wantErr = %v", root, tc.subPath, err, tc.wantErr)
		}
	}
}

func TestCanonicalizeRepoRelative(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		repoName string
		branch   string
		wantPath string
		wantOk   bool
	}{
		{
			name:     "already relative",
			path:     "cmd/sync/main.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "cmd/sync/main.go",
			wantOk:   true,
		},
		{
			name:     "single workspace prefix",
			path:     "code/repos/tool_zentao/main/cmd/sync/main.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "cmd/sync/main.go",
			wantOk:   true,
		},
		{
			name:     "worktree suffix prefix",
			path:     "code/repos/tool_zentao/worktrees/backend/cmd/sync/main.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "cmd/sync/main.go",
			wantOk:   true,
		},
		{
			name:     "doubled prefix (call-131)",
			path:     "code/repos/tool_zentao/main/code/repos/tool_zentao/main/cmd/sync/main.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "cmd/sync/main.go",
			wantOk:   true,
		},
		{
			name:     "foreign repo prefix - rejected",
			path:     "code/repos/other_repo/main/cmd/sync/main.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "",
			wantOk:   false,
		},
		{
			name:     "traversal escape - rejected",
			path:     "code/repos/tool_zentao/main/../../evil.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "",
			wantOk:   false,
		},
		{
			name:     "traversal inside - rejected",
			path:     "cmd/../evil.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "",
			wantOk:   false,
		},
		{
			name:     "git diff a/ prefix - resolved",
			path:     "a/cmd/sync/main.go",
			repoName: "tool_zentao",
			branch:   "main",
			wantPath: "cmd/sync/main.go",
			wantOk:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := CanonicalizeRepoRelative(tc.path, tc.repoName, tc.branch)
			if ok != tc.wantOk {
				t.Fatalf("CanonicalizeRepoRelative() ok = %v, want %v", ok, tc.wantOk)
			}
			if ok && got != tc.wantPath {
				t.Errorf("CanonicalizeRepoRelative() got = %q, want %q", got, tc.wantPath)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"hello_world", "hello-world"},
		{"Add billing reconcile schema!", "add-billing-reconcile-schema"},
		{"Special!@#Characters$%^&*", "special-characters"},
		{"A very long title that should be truncated because it is more than 30 characters long", "a-very-long-title-that-should"},
	}

	for _, tc := range tests {
		got := Slugify(tc.input)
		if got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDeriveBranchName(t *testing.T) {
	taskID := "787ee609-85d5-4a04-8182-33e43df13685"
	title := "Add billing reconcile schema"

	got := DeriveBranchName(taskID, title)
	want := "feature/add-billing-reconcile-schema-787ee609"
	if got != want {
		t.Errorf("DeriveBranchName(%q, %q) = %q, want %q", taskID, title, got, want)
	}

	gotRole := DeriveRoleBranchName(taskID, title, "be")
	wantRole := "feature/add-billing-reconcile-schema-787ee609-be"
	if gotRole != wantRole {
		t.Errorf("DeriveRoleBranchName(%q, %q, \"be\") = %q, want %q", taskID, title, gotRole, wantRole)
	}
}

