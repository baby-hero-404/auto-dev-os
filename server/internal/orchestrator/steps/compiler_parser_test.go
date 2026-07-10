package steps

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestParseCompilerErrorFiles(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "basic go error with col and line",
			output: "internal/model/commit.go:21:1: syntax error",
			want:   []string{"internal/model/commit.go"},
		},
		{
			name:   "go error with line only",
			output: "server/main.go:50: undefined: fmt.Printf",
			want:   []string{"server/main.go"},
		},
		{
			name: "multi-line compilation output",
			output: `# github.com/auto-code-os/auto-code-os/server/internal/orchestrator
internal/orchestrator/worker.go:338:43: undefined: provider
internal/orchestrator/steps/fix.go:12:1: syntax error`,
			want: []string{"internal/orchestrator/worker.go", "internal/orchestrator/steps/fix.go"},
		},
		{
			name:   "no matches",
			output: "some generic error message without file info",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCompilerErrorFiles(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCompilerErrorFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockCompilerTaskRepository struct {
	task *models.Task
}

func (m *mockCompilerTaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	return m.task, nil
}

func (m *mockCompilerTaskRepository) Update(ctx context.Context, id string, input models.UpdateTaskInput) (*models.Task, error) {
	if input.Analysis != nil {
		m.task.Analysis = input.Analysis
	}
	return m.task, nil
}

func TestUpdateAffectedFilesWithErrors(t *testing.T) {
	task := &models.Task{
		ID:       "task-123",
		Analysis: json.RawMessage(`{"affected_files": [{"file": "existing.go", "confidence": 1.0, "reason": "existing"}]}`),
	}
	repo := &mockCompilerTaskRepository{task: task}

	err := errors.New("compilation failed:\ninternal/model/commit.go:21:1: syntax error\nexisting.go:10:1: syntax error")
	updateAffectedFilesWithErrors(context.Background(), "task-123", repo, task, err)

	var analysis models.TaskAnalysis
	if errUnmarshal := json.Unmarshal(task.Analysis, &analysis); errUnmarshal != nil {
		t.Fatalf("failed to unmarshal task analysis: %v", errUnmarshal)
	}

	if len(analysis.AffectedFiles) != 2 {
		t.Fatalf("expected 2 affected files, got %d: %+v", len(analysis.AffectedFiles), analysis.AffectedFiles)
	}

	if analysis.AffectedFiles[0].File != "existing.go" {
		t.Errorf("expected first file to be existing.go, got %s", analysis.AffectedFiles[0].File)
	}

	if analysis.AffectedFiles[1].File != "internal/model/commit.go" {
		t.Errorf("expected second file to be internal/model/commit.go, got %s", analysis.AffectedFiles[1].File)
	}
}
