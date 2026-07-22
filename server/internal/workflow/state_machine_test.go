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
		{"todo to merged", models.TaskStatusTodo, models.TaskStatusMerged, true},
		{"merged to todo", models.TaskStatusMerged, models.TaskStatusTodo, true},
		{"failed to todo", models.TaskStatusFailed, models.TaskStatusTodo, false},
		{"todo to context_loading", models.TaskStatusTodo, models.TaskStatusContextLoading, false},
		{"context_loading to analyzing", models.TaskStatusContextLoading, models.TaskStatusAnalyzing, false},
		{"testing to pr_ready", models.TaskStatusTesting, models.TaskStatusPrReady, false},
		{"pr_ready to human_review", models.TaskStatusPrReady, models.TaskStatusHumanReview, false},
		{"pr_ready to merged", models.TaskStatusPrReady, models.TaskStatusMerged, false},
		{"todo to pr_ready", models.TaskStatusTodo, models.TaskStatusPrReady, true},
		{"coding to pr_ready", models.TaskStatusCoding, models.TaskStatusPrReady, true},
		{"failed to context_loading", models.TaskStatusFailed, models.TaskStatusContextLoading, false},
		{"context_loading to reviewing", models.TaskStatusContextLoading, models.TaskStatusReviewing, false},
		{"context_loading to testing", models.TaskStatusContextLoading, models.TaskStatusTesting, false},
		{"context_loading to pr_ready", models.TaskStatusContextLoading, models.TaskStatusPrReady, false},
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

func TestComplexityWorkflowDefinitions(t *testing.T) {
	runners := map[string]StepFunc{}
	easy := EasyWorkflow(runners)
	if len(easy.Steps) != 5 {
		t.Fatalf("expected easy workflow to have 5 steps, got %d", len(easy.Steps))
	}
	if easy.Steps[2].ID != StepCodeBackend || easy.Steps[2].DependsOn[0] != StepAnalyze {
		t.Fatalf("unexpected easy workflow code step: %#v", easy.Steps[2])
	}

	hard := HardWorkflow(runners, nil)
	if len(hard.Steps) != 10 {
		t.Fatalf("expected hard workflow to have 10 steps, got %d", len(hard.Steps))
	}
	if hard.Steps[3].ID != StepCodeBackend || hard.Steps[4].ID != StepCodeFrontend {
		t.Fatalf("expected hard workflow backend/frontend fan-out, got %#v", hard.Steps)
	}
	if len(hard.Steps[5].DependsOn) != 2 {
		t.Fatalf("expected merge to depend on both code steps, got %#v", hard.Steps[5].DependsOn)
	}
}

func TestCLISpecFirstWorkflow(t *testing.T) {
	runners := map[string]StepFunc{}
	def := CLISpecFirstWorkflow(runners)
	if len(def.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(def.Steps))
	}
	wantOrder := []string{StepCLIAnalyze, StepCLISpec, StepCLIImplement, StepCLIMR}
	for i, s := range def.Steps {
		if s.ID != wantOrder[i] {
			t.Errorf("step %d: got %q, want %q", i, s.ID, wantOrder[i])
		}
		if i == 0 {
			if len(s.DependsOn) != 0 {
				t.Errorf("expected first step to have no dependencies, got %#v", s.DependsOn)
			}
			continue
		}
		if len(s.DependsOn) != 1 || s.DependsOn[0] != wantOrder[i-1] {
			t.Errorf("step %q: expected dependency on %q, got %#v", s.ID, wantOrder[i-1], s.DependsOn)
		}
	}
	if _, err := ValidateDAG(def); err != nil {
		t.Fatalf("expected valid DAG, got %v", err)
	}
}
