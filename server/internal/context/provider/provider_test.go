package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegrationLatencyAndLeakage(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDb := filepath.Join(tmpDir, "cache.db")

	// 1. Generate a mock repository with 20 files
	for i := 0; i < 20; i++ {
		content := fmt.Sprintf(`
package main

func MyFunc%d() {
	// A secret internal logic body that should never leak out
	secretLogicBody_%d := true
	_ = secretLogicBody_%d
}
`, i, i, i)
		err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file_%d.go", i)), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	provider, err := NewProvider(tmpDir, cacheDb)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()

	// 2. Cold Start Latency Test (100% Cache miss)
	start := time.Now()
	repoMap, err := provider.GetRepoMap(context.Background(), []string{}, 2048)
	if err != nil {
		t.Fatal(err)
	}
	coldLatency := time.Since(start)

	if coldLatency > 10*time.Second {
		t.Errorf("Cold start latency extremely slow: %v", coldLatency)
	}

	// 3. Hot Cache Latency Test (100% Cache hit)
	start = time.Now()
	repoMapHot, err := provider.GetRepoMap(context.Background(), []string{}, 2048)
	if err != nil {
		t.Fatal(err)
	}
	hotLatency := time.Since(start)

	if hotLatency > 150*time.Millisecond {
		t.Errorf("Hot cache latency failed target (<150ms): %v", hotLatency)
	}

	// 4. Data Leakage Test
	if strings.Contains(repoMap, "secretLogicBody") {
		t.Errorf("DATA LEAKAGE DETECTED: internal function body found in repo map:\n%s", repoMap)
	}
	
	// 5. Reproducibility
	if repoMap != repoMapHot {
		t.Errorf("Hot cache output does not match cold run output.")
	}
}

func TestGetRepoMapIsolationAndPathCorrection(t *testing.T) {
	tmpGlobalRoot := t.TempDir()
	cacheDb := filepath.Join(tmpGlobalRoot, "cache.db")

	// Create task workspace directory inside global root
	taskID := "21065286-e2bc-43a1-8c4f-d436c8f3f046"
	taskWS := filepath.Join(tmpGlobalRoot, taskID)
	err := os.MkdirAll(taskWS, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Create workspace-internal files that should NOT be scanned/mapped
	internalFiles := []string{
		filepath.Join(taskWS, "task.json"),
		filepath.Join(taskWS, ".workspace.lock"),
		filepath.Join(taskWS, "artifacts", "workflow_timeline.jsonl"),
		filepath.Join(taskWS, "logs", "llm", "call-001-analyze", "prompt.md"),
	}
	for _, file := range internalFiles {
		err := os.MkdirAll(filepath.Dir(file), 0755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(file, []byte("internal content func LeakBody() {}"), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 2. Create actual repository code checkout that SHOULD be scanned
	codeRepoDir := filepath.Join(taskWS, "code", "repos", "tool_zentao", "main")
	err = os.MkdirAll(codeRepoDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	goCodeFile := filepath.Join(codeRepoDir, "main.go")
	goCodeContent := `
package main

func MyActualLogic() {
	// target code function
}
`
	err = os.WriteFile(goCodeFile, []byte(goCodeContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Create another task directory inside global root to verify isolation
	otherTaskWS := filepath.Join(tmpGlobalRoot, "4fb3c8ce-abec-4244-8147-abc79402fdfd")
	otherCodeRepoDir := filepath.Join(otherTaskWS, "code", "repos", "other_tool", "main")
	err = os.MkdirAll(otherCodeRepoDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	otherGoFile := filepath.Join(otherCodeRepoDir, "helper.go")
	err = os.WriteFile(otherGoFile, []byte("package helper\nfunc OtherLogic() {}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize provider with the global root
	provider, err := NewProvider(tmpGlobalRoot, cacheDb)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()

	// 4. Test safety check: calling GetRepoMap with global root (or no WorkspaceRootKey in ctx)
	// should skip scanning and return empty string
	ctxGlobal := context.Background()
	globalRepoMap, err := provider.GetRepoMap(ctxGlobal, []string{}, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if globalRepoMap != "" {
		t.Errorf("Expected global scan to return empty string, but got:\n%s", globalRepoMap)
	}

	// 5. Test specific task scan: setting provider.WorkspaceRootKey to taskWS
	ctxTask := context.WithValue(context.Background(), WorkspaceRootKey, taskWS)
	taskRepoMap, err := provider.GetRepoMap(ctxTask, []string{}, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// 6. Validations
	// Should contain the target go file with path starting with code/repos/
	expectedPath := "code/repos/tool_zentao/main/main.go"
	if !strings.Contains(taskRepoMap, expectedPath) {
		t.Errorf("Expected repo map to contain relative path %q, but got:\n%s", expectedPath, taskRepoMap)
	}

	// Should contain the function definition of the repository code
	if !strings.Contains(taskRepoMap, "MyActualLogic") {
		t.Errorf("Expected repo map to contain target function 'MyActualLogic', but got:\n%s", taskRepoMap)
	}

	// Should NOT contain the internal workspace metadata files
	for _, file := range []string{"task.json", "workflow_timeline.jsonl", "prompt.md", "LeakBody"} {
		if strings.Contains(taskRepoMap, file) {
			t.Errorf("DATA LEAKAGE: Repo map contains internal workspace metadata file reference or content: %q", file)
		}
	}

	// Should NOT contain other task workspace files (isolation check)
	if strings.Contains(taskRepoMap, "other_tool") || strings.Contains(taskRepoMap, "OtherLogic") {
		t.Errorf("ISOLATION FAILURE: Repo map contains files/content from other task workspace: %s", taskRepoMap)
	}
}

func TestGetRepoMapTokenPruningAndBuffing(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDb := filepath.Join(tmpDir, "cache.db")
	taskWS := filepath.Join(tmpDir, "task1")
	codeRepoDir := filepath.Join(taskWS, "code", "repos", "app")
	err := os.MkdirAll(codeRepoDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create 10 files
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("package main\n\nfunc Function%d() {}\n", i)
		err := os.WriteFile(filepath.Join(codeRepoDir, fmt.Sprintf("file_%d.go", i)), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	provider, err := NewProvider(tmpDir, cacheDb)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()

	ctx := context.WithValue(context.Background(), WorkspaceRootKey, taskWS)

	// Test 1: Restrict budget to very small tokens, should return minimal string
	maxTokensSmall := 20
	repoMapSmall, err := provider.GetRepoMap(ctx, []string{}, maxTokensSmall)
	if err != nil {
		t.Fatal(err)
	}
	if len(repoMapSmall) > maxTokensSmall*8 { // Assuming approx 1 token = 4-8 chars
		t.Errorf("Repo map exceeded small token budget. Got %d bytes", len(repoMapSmall))
	}

	// Test 2: Task Dependency Buffing
	// We specifically buff file_9.go which normally has the same PageRank as others.
	activeFiles := []string{"code/repos/app/file_9.go"}
	repoMapBuffed, err := provider.GetRepoMap(ctx, activeFiles, maxTokensSmall)
	if err != nil {
		t.Fatal(err)
	}

	// Because of buffing, file_9.go MUST be included even in a highly restricted token budget
	if !strings.Contains(repoMapBuffed, "file_9.go") {
		t.Errorf("Buffed task dependency (file_9.go) was not prioritized in the pruned repo map:\n%s", repoMapBuffed)
	}
}


