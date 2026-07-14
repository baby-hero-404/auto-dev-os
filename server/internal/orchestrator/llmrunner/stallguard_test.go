package llmrunner

import (
	"context"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
)

func TestStallGuard_CheckNoOpSearchReplace(t *testing.T) {
	sg := newStallGuard()

	// 1. Same search and replace content
	argsJSON := `{"path":"main.go","search":"func main() {}","replace":"func main() {}"}`
	result, blocked := sg.Check("search_replace", argsJSON)
	if !blocked {
		t.Fatal("expected search_replace with identical search/replace to be blocked")
	}
	if !strings.Contains(result, "no-op") {
		t.Errorf("expected no-op error message, got: %s", result)
	}

	// 2. Different search and replace content
	argsJSONDiff := `{"path":"main.go","search":"func main() {}","replace":"func main() { fmt.Println() }"}`
	_, blockedDiff := sg.Check("search_replace", argsJSONDiff)
	if blockedDiff {
		t.Fatal("expected search_replace with different search/replace to NOT be blocked")
	}
}

func TestStallGuard_CheckReadOnlyRepeats(t *testing.T) {
	sg := newStallGuard()

	// 1. First execution of list_files should pass and be recorded
	argsJSON := `{"directory":"server"}`
	_, blocked := sg.Check("list_files", argsJSON)
	if blocked {
		t.Fatal("expected list_files to not be blocked on first execution")
	}

	sg.RecordSuccess("list_files", argsJSON, 1)

	// 2. Second execution with identical args should be blocked
	result, blocked := sg.Check("list_files", argsJSON)
	if !blocked {
		t.Fatal("expected identical list_files run to be blocked")
	}
	if !strings.Contains(result, "You already ran list_files") {
		t.Errorf("expected duplicate read-only message, got: %s", result)
	}

	// 3. Different args should not be blocked
	argsJSONDiff := `{"directory":"pkg"}`
	_, blockedDiff := sg.Check("list_files", argsJSONDiff)
	if blockedDiff {
		t.Fatal("expected different list_files run to NOT be blocked")
	}
}

func TestRunToolLoop_StallGuardNoOpEdit(t *testing.T) {
	calls := 0
	toolExecs := 0

	cfg := ToolLoopConfig{
		Messages:      []llm.Message{{Role: "user", Content: "run"}},
		Tools:         []llm.ToolDefinition{{Name: "search_replace"}},
		MaxIterations: 3,
		Chat: func(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (*llm.Response, error) {
			calls++
			// Model keeps outputting a no-op search_replace call
			return &llm.Response{
				ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "search_replace", Arguments: `{"path":"main.go","search":"hello","replace":"hello"}`}},
			}, nil
		},
		ExecuteTool: func(ctx context.Context, name, argumentsJSON string) (string, error) {
			toolExecs++
			return "ok", nil
		},
	}

	_, messages, result, err := RunToolLoop(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected a budget exhaustion error since LLM was stuck in no-op loop")
	}
	if !strings.Contains(err.Error(), "exceeded max iterations") {
		t.Fatalf("expected iteration exhaustion error, got: %v", err)
	}

	if toolExecs != 0 {
		t.Errorf("expected ExecuteTool to NOT be called for blocked no-op edits, got %d executions", toolExecs)
	}

	// Ensure no-op edits were NOT added to EditsApplied
	if result != nil && len(result.EditsApplied) > 0 {
		t.Errorf("expected zero edits applied in result, got: %v", result.EditsApplied)
	}
	if result != nil && result.Partial {
		t.Error("expected Partial to be false since zero edits actually succeeded")
	}

	// Verify the nudge feedback was sent back
	foundNudge := false
	for _, m := range messages {
		if m.Role == "tool" && strings.Contains(m.Content, "no-op") {
			foundNudge = true
			break
		}
	}
	if !foundNudge {
		t.Fatal("expected the no-op corrective message to be in message history")
	}
}

func TestStallGuard_InvalidationOnEdit(t *testing.T) {
	sg := newStallGuard()

	// 1. Record build and test runs
	buildArgs := `{"target":"server"}`
	testArgs := `{"package":"./..."}`
	sg.RecordSuccess("run_build", buildArgs, 1)
	sg.RecordSuccess("run_tests", testArgs, 2)

	// 2. Verify they are blocked when run again
	_, blockedBuild := sg.Check("run_build", buildArgs)
	if !blockedBuild {
		t.Fatal("expected run_build to be blocked on duplicate call")
	}
	_, blockedTest := sg.Check("run_tests", testArgs)
	if !blockedTest {
		t.Fatal("expected run_tests to be blocked on duplicate call")
	}

	// 3. Record an edit tool success (e.g. search_replace or create_file)
	sg.RecordSuccess("search_replace", `{"path":"main.go","search":"a","replace":"b"}`, 3)

	// 4. Verify they are NO LONGER blocked
	_, blockedBuildAfter := sg.Check("run_build", buildArgs)
	if blockedBuildAfter {
		t.Fatal("expected run_build to be invalidated and allowed after successful edit tool call")
	}
	_, blockedTestAfter := sg.Check("run_tests", testArgs)
	if blockedTestAfter {
		t.Fatal("expected run_tests to be invalidated and allowed after successful edit tool call")
	}
}
