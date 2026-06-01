package workflow

import (
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestValidateJobTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"same status", models.WorkflowJobStatusQueued, models.WorkflowJobStatusQueued, false},
		{"queued to running", models.WorkflowJobStatusQueued, models.WorkflowJobStatusRunning, false},
		{"queued to done", models.WorkflowJobStatusQueued, models.WorkflowJobStatusDone, true},
		{"running to done", models.WorkflowJobStatusRunning, models.WorkflowJobStatusDone, false},
		{"running to queued", models.WorkflowJobStatusRunning, models.WorkflowJobStatusQueued, true},
		{"failed to queued", models.WorkflowJobStatusFailed, models.WorkflowJobStatusQueued, false},
		{"done to running", models.WorkflowJobStatusDone, models.WorkflowJobStatusRunning, true},
		{"unknown status", "unknown", models.WorkflowJobStatusRunning, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJobTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJobTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTaskTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"same status", models.TaskStatusTodo, models.TaskStatusTodo, false},
		{"todo to analyzing", models.TaskStatusTodo, models.TaskStatusAnalyzing, false},
		{"todo to completed", models.TaskStatusTodo, models.TaskStatusCompleted, true},
		{"completed to todo", models.TaskStatusCompleted, models.TaskStatusTodo, true},
		{"failed to todo", models.TaskStatusFailed, models.TaskStatusTodo, false},
		{"unknown status", "unknown", models.TaskStatusTodo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaskTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTaskTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
