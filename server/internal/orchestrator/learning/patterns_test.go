package learning

import (
	"reflect"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestExtractChangedFiles(t *testing.T) {
	t.Run("parses a files_changed line", func(t *testing.T) {
		content := "status: success\nfiles_changed: [main.go utils.go]\nduration: 1.2s"
		got := extractChangedFiles(content)
		want := []string{"main.go", "utils.go"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("no files_changed marker returns nil", func(t *testing.T) {
		if got := extractChangedFiles("status: success\nduration: 1.2s"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty files_changed list returns nil", func(t *testing.T) {
		if got := extractChangedFiles("files_changed: []"); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestTaskToolCallSummary(t *testing.T) {
	taskID := "task-1"

	t.Run("4 distinct tool calls across steps", func(t *testing.T) {
		memories := []models.EpisodicMemory{
			{
				TaskID:   &taskID,
				Category: models.MemoryCategoryObservation,
				Tags:     []string{"workflow", "plan", "success"},
				Content:  "plan created",
			},
			{
				TaskID:   &taskID,
				Category: models.MemoryCategorySuccess,
				Tags:     []string{"workflow", "code_backend", "success"},
				Content:  "files_changed: [main.go utils.go]",
			},
			{
				TaskID:   &taskID,
				Category: models.MemoryCategorySuccess,
				Tags:     []string{"workflow", "code_frontend", "success"},
				Content:  "files_changed: [handler.go router.go]",
			},
		}

		stepSequence, toolCalls := taskToolCallSummary(taskID, memories)

		if len(stepSequence) != 3 {
			t.Errorf("expected 3 distinct steps, got %d: %v", len(stepSequence), stepSequence)
		}
		if len(toolCalls) != 4 {
			t.Errorf("expected 4 distinct tool calls, got %d: %v", len(toolCalls), toolCalls)
		}
	})

	t.Run("1 tool call is below threshold", func(t *testing.T) {
		memories := []models.EpisodicMemory{
			{
				TaskID:   &taskID,
				Category: models.MemoryCategorySuccess,
				Tags:     []string{"workflow", "code_backend", "success"},
				Content:  "files_changed: [main.go]",
			},
		}

		_, toolCalls := taskToolCallSummary(taskID, memories)
		if len(toolCalls) != 1 {
			t.Errorf("expected 1 distinct tool call, got %d: %v", len(toolCalls), toolCalls)
		}
	})

	t.Run("many distinct workflow steps but no file edits still yields zero tool calls", func(t *testing.T) {
		// Regression test for the bug where SuggestSkillFromTask counted distinct
		// workflow steps (mem.Tags[1]) as a stand-in for tool calls. A task with
		// many steps (plan, review, verify) but no edit tool calls should NOT
		// be treated as having 3+ tool calls.
		memories := []models.EpisodicMemory{
			{TaskID: &taskID, Category: models.MemoryCategoryObservation, Tags: []string{"workflow", "plan", "success"}, Content: "planned"},
			{TaskID: &taskID, Category: models.MemoryCategoryObservation, Tags: []string{"workflow", "review", "success"}, Content: "reviewed"},
			{TaskID: &taskID, Category: models.MemoryCategoryObservation, Tags: []string{"workflow", "verify", "success"}, Content: "verified"},
		}

		stepSequence, toolCalls := taskToolCallSummary(taskID, memories)
		if len(stepSequence) != 3 {
			t.Errorf("expected 3 distinct steps, got %d", len(stepSequence))
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 distinct tool calls (no files_changed data), got %d: %v", len(toolCalls), toolCalls)
		}
	})

	t.Run("duplicate files across steps are deduplicated", func(t *testing.T) {
		memories := []models.EpisodicMemory{
			{TaskID: &taskID, Category: models.MemoryCategorySuccess, Tags: []string{"workflow", "code_backend", "success"}, Content: "files_changed: [main.go]"},
			{TaskID: &taskID, Category: models.MemoryCategorySuccess, Tags: []string{"workflow", "fix_lint", "success"}, Content: "files_changed: [main.go]"},
		}

		_, toolCalls := taskToolCallSummary(taskID, memories)
		if len(toolCalls) != 1 {
			t.Errorf("expected deduplication to 1 distinct file, got %d: %v", len(toolCalls), toolCalls)
		}
	})

	t.Run("memories from other tasks are ignored", func(t *testing.T) {
		other := "task-2"
		memories := []models.EpisodicMemory{
			{TaskID: &other, Category: models.MemoryCategorySuccess, Tags: []string{"workflow", "code_backend", "success"}, Content: "files_changed: [a.go b.go c.go]"},
		}

		stepSequence, toolCalls := taskToolCallSummary(taskID, memories)
		if len(stepSequence) != 0 || len(toolCalls) != 0 {
			t.Errorf("expected no data for unrelated task, got steps=%v toolCalls=%v", stepSequence, toolCalls)
		}
	})
}
