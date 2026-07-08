package steps

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestShouldSkipFrontend(t *testing.T) {
	tests := []struct {
		name     string
		analysis models.TaskAnalysis
		subtasks map[string][]string
		expected bool
	}{
		{
			name:     "BackendOnly",
			analysis: models.TaskAnalysis{PrimaryCategory: "backend"},
			subtasks: map[string][]string{"backend": {"task1"}},
			expected: true,
		},
		{
			name:     "DatabaseCategory",
			analysis: models.TaskAnalysis{PrimaryCategory: "database"},
			subtasks: map[string][]string{},
			expected: true,
		},
		{
			name:     "HasFrontendTasks",
			analysis: models.TaskAnalysis{PrimaryCategory: "frontend"},
			subtasks: map[string][]string{
				"backend":  {"be task"},
				"frontend": {"fe task"},
			},
			expected: false,
		},
		{
			name:     "FrontendTasksOverrideBackendCategory",
			analysis: models.TaskAnalysis{PrimaryCategory: "backend"},
			subtasks: map[string][]string{
				"backend":  {"be task"},
				"frontend": {"fe task"},
			},
			expected: false,
		},
		{
			name:     "BackendCategory_WithSubtasks_KeepsFrontend",
			analysis: models.TaskAnalysis{PrimaryCategory: "backend"},
			subtasks: map[string][]string{
				"frontend": {"fe task"},
			},
			expected: false,
		},
		{
			name: "NoSubtasksButFrontendFiles",
			analysis: models.TaskAnalysis{
				PrimaryCategory: "",
				AffectedFiles:   []models.AffectedFile{{File: "web/src/App.tsx"}, {File: "server/main.go"}},
			},
			subtasks: map[string][]string{"backend": {"be task"}},
			expected: false,
		},
		{
			name: "NoSubtasksNoFiles",
			analysis: models.TaskAnalysis{
				PrimaryCategory: "",
				AffectedFiles:   []models.AffectedFile{{File: "server/main.go"}, {File: "internal/repo.go"}},
			},
			subtasks: map[string][]string{"backend": {"be task"}},
			expected: true,
		},
		{
			name: "UnknownCategory_NoSubtasks_Skips",
			analysis: models.TaskAnalysis{
				PrimaryCategory: "mobile",
				AffectedFiles:   []models.AffectedFile{{File: "server/main.go"}},
			},
			subtasks: map[string][]string{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipFrontend(tc.analysis, tc.subtasks); got != tc.expected {
				t.Errorf("expected skip=%v, got skip=%v", tc.expected, got)
			}
		})
	}
}

func TestPlanStep_NoLLMCall(t *testing.T) {
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		ExecutionPhases:   []models.ExecutionPhase{{Phase: "Backend", Tasks: []string{"Task 1"}}},
	}
	analysisJSON, _ := json.Marshal(analysis)

	task := &models.Task{
		ID:         "task-1",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}
	log := &mockLogger{}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, // LLMRunner is nil — must not panic
		nil,
		nil,
		&mockStatusUpdater{},
		log,
		&mockCheckpointLister{},
		8.0,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Verify subtasks are populated
	subtasks, ok := result["subtasks"].(map[string][]string)
	if !ok {
		t.Fatalf("expected subtasks in result, got %T", result["subtasks"])
	}
	if len(subtasks["backend"]) != 1 {
		t.Errorf("expected 1 backend subtask, got %d", len(subtasks["backend"]))
	}
	// Verify skip_frontend
	if skip, ok := result["skip_frontend"].(bool); !ok || !skip {
		t.Errorf("expected skip_frontend=true for backend category, got %v", result["skip_frontend"])
	}
}

func TestPlanStep_BranchSetup(t *testing.T) {
	analysis := models.TaskAnalysis{Complexity: "medium", ExecutionPhases: []models.ExecutionPhase{{Phase: "Backend", Tasks: []string{"t1"}}}}
	analysisJSON, _ := json.Marshal(analysis)
	task := &models.Task{
		ID:         "task-2",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}
	worktree := &mockWorktreeManager{}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil,
		worktree,
		&mockStepWorkspaceLoader{},
		&mockStatusUpdater{},
		&mockLogger{},
		&mockCheckpointLister{},
		8.0,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !worktree.setupCalled {
		t.Error("expected worktree.SetupRoleBranches to be called")
	}
	branches, ok := result["branches"].(map[string]string)
	if !ok {
		t.Fatal("expected branches in result")
	}
	if !strings.Contains(branches["integration"], "feature/task-2") {
		t.Errorf("unexpected integration branch: %s", branches["integration"])
	}
}

func TestPlanStep_OutputStructure(t *testing.T) {
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "frontend",
		ExecutionPhases: []models.ExecutionPhase{
			{Phase: "Backend", Tasks: []string{"be1"}},
			{Phase: "UI Components", Tasks: []string{"fe1", "fe2"}},
		},
	}
	analysisJSON, _ := json.Marshal(analysis)
	task := &models.Task{
		ID:         "task-3",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}

	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil,
		&mockStatusUpdater{},
		&mockLogger{},
		&mockCheckpointLister{},
		8.0,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check subtasks structure
	subtasks, ok := result["subtasks"].(map[string][]string)
	if !ok {
		t.Fatalf("subtasks wrong type: %T", result["subtasks"])
	}
	if len(subtasks["backend"]) != 1 || len(subtasks["frontend"]) != 1 {
		t.Errorf("unexpected subtask counts: be=%d fe=%d", len(subtasks["backend"]), len(subtasks["frontend"]))
	}

	// Check skip_frontend = false for frontend category with frontend tasks
	if skip, _ := result["skip_frontend"].(bool); skip {
		t.Error("expected skip_frontend=false for frontend category with frontend tasks")
	}

	// Check execution_phases propagated
	plan, ok := result["execution_phases"].([]models.ExecutionPhase)
	if !ok {
		t.Fatalf("execution_phases wrong type: %T", result["execution_phases"])
	}
	if len(plan) != 2 || len(plan[0].Tasks) != 1 || plan[0].Tasks[0] != "be1" {
		t.Errorf("unexpected execution_phases: %v", plan)
	}
}

func TestPlanStep_EmitsLogs(t *testing.T) {
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		ExecutionPhases: []models.ExecutionPhase{{Phase: "Backend", Tasks: []string{"t1"}}},
	}
	analysisJSON, _ := json.Marshal(analysis)
	task := &models.Task{
		ID:         "task-log",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}
	log := &mockLogger{}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil,
		&mockStatusUpdater{},
		log,
		&mockCheckpointLister{},
		8.0,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"mapping ExecutionUnits",
		"backend units",
		"skip_frontend",
		"setting up role branches",
		"orchestration checkpoint complete",
	}
	for _, phrase := range expectedPhrases {
		found := false
		for _, msg := range log.messages {
			if strings.Contains(msg, phrase) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected log message containing %q, got messages: %v", phrase, log.messages)
		}
	}
}

func TestPlanStep_EasyTaskSkipped(t *testing.T) {
	task := &models.Task{
		ID:         "task-easy",
		Complexity: models.TaskComplexityEasy,
	}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil,
		&mockStatusUpdater{}, // Need to add StatusUpdater to fix compilation (not really needed for skip though)
		&mockLogger{},
		&mockCheckpointLister{},
		8.0,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status, _ := result["status"].(string); status != "skipped" {
		t.Errorf("expected status=skipped for easy task, got %v", result)
	}
}

func TestPlanStep_EmptyAnalysisFallback(t *testing.T) {
	task := &models.Task{
		ID:         "task-empty",
		Complexity: models.TaskComplexityMedium,
		Analysis:   nil,
	}
	statusMock := &mockStatusUpdater{}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil,
		statusMock,
		&mockLogger{},
		&mockCheckpointLister{},
		8.0,
	)

	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should succeed with empty subtasks
	subtasks, ok := result["subtasks"].(map[string][]string)
	if !ok {
		t.Fatalf("subtasks wrong type: %T", result["subtasks"])
	}
	if len(subtasks) != 0 {
		t.Errorf("expected empty subtasks for nil analysis, got %v", subtasks)
	}
	if statusMock.lastStatus != models.TaskStatusCoding {
		t.Errorf("expected status transition to coding, got %s", statusMock.lastStatus)
	}
}

func TestPlanStep_ValidationFailure_AutoRetry(t *testing.T) {
	// A unit that violates Max Cost (> 8) or has a cycle.
	// Let's create one with a cyclic dependency.
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		TasksMD:         "## Backend\n- [ ] Task 1\n",
		ExecutionUnits: []models.ExecutionUnit{
			{
				ID:           "unit1",
				Objective:    "Loop 1",
				Dependencies: []string{"unit2"},
				ExecutionProfile: models.ExecutionProfile{
					Agent: "backend",
				},
			},
			{
				ID:           "unit2",
				Objective:    "Loop 2",
				Dependencies: []string{"unit1"},
				ExecutionProfile: models.ExecutionProfile{
					Agent: "backend",
				},
			},
		},
		RetryCount: 0,
	}
	analysisJSON, _ := json.Marshal(analysis)
	task := &models.Task{
		ID:         "task-retry",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}

	mockRepo := &mockTaskReader{task: task}
	mockCps := &mockCheckpointLister{}

	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		mockRepo,
		nil, nil, nil,
		&mockStatusUpdater{},
		&mockLogger{},
		mockCps,
		8.0,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error due to cycle validation failure")
	}

	// Should be ErrGraphChanged to trigger retry
	if err != workflow.ErrGraphChanged {
		t.Errorf("expected workflow.ErrGraphChanged, got %v", err)
	}

	// Verify retry count was incremented and saved
	var updatedAnalysis models.TaskAnalysis
	_ = json.Unmarshal(task.Analysis, &updatedAnalysis)
	if updatedAnalysis.RetryCount != 1 {
		t.Errorf("expected RetryCount to be 1, got %d", updatedAnalysis.RetryCount)
	}
}

func TestPlanStep_ValidationFailure_ExceededPause(t *testing.T) {
	// Cycle dependency and RetryCount already = 2 (max retries reached).
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		TasksMD:         "## Backend\n- [ ] Task 1\n",
		ExecutionUnits: []models.ExecutionUnit{
			{
				ID:           "unit1",
				Objective:    "Loop 1",
				Dependencies: []string{"unit2"},
				ExecutionProfile: models.ExecutionProfile{
					Agent: "backend",
				},
			},
			{
				ID:           "unit2",
				Objective:    "Loop 2",
				Dependencies: []string{"unit1"},
				ExecutionProfile: models.ExecutionProfile{
					Agent: "backend",
				},
			},
		},
		RetryCount: 2,
	}
	analysisJSON, _ := json.Marshal(analysis)
	task := &models.Task{
		ID:         "task-pause",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}

	mockRepo := &mockTaskReader{task: task}
	mockCps := &mockCheckpointLister{}
	statusMock := &mockStatusUpdater{}

	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		mockRepo,
		nil, nil, nil,
		statusMock,
		&mockLogger{},
		mockCps,
		8.0,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err == nil {
		t.Fatal("expected error")
	}

	// Should be workflow.ErrPaused
	if !strings.Contains(err.Error(), "workflow paused") {
		t.Errorf("expected paused error, got %v", err)
	}

	// Verify SpecStatus is updated to paused and Task Status to spec_review
	if task.SpecStatus != "paused" {
		t.Errorf("expected SpecStatus to be paused, got %v", task.SpecStatus)
	}
	if statusMock.lastStatus != models.TaskStatusSpecReview {
		t.Errorf("expected task status to transition to spec_review, got %s", statusMock.lastStatus)
	}
}

func TestPlanStep_SkipWhenExecutionUnitsAlreadyProvided(t *testing.T) {
	// 1. Path where execution units are already fully provided
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		ExecutionUnits: []models.ExecutionUnit{
			{
				ID:        "unit1",
				Objective: "Obj 1",
				Tasks:     []string{"Task 1"},
				ExecutionProfile: models.ExecutionProfile{
					Agent: "backend",
				},
			},
		},
	}
	analysisJSON, _ := json.Marshal(analysis)
	task := &models.Task{
		ID:         "task-skip-provided",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisJSON,
	}

	log := &mockLogger{}
	step := NewPlanStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil,
		&mockStatusUpdater{},
		log,
		&mockCheckpointLister{},
		8.0,
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the log was emitted
	found := false
	for _, msg := range log.messages {
		if strings.Contains(msg, "Plan step skipped — execution units already provided by analyze step") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected skip log message, got: %v", log.messages)
	}

	// 2. Path where execution units are not fully provided (tasks are empty)
	analysisEmpty := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		ExecutionUnits: []models.ExecutionUnit{
			{
				ID:        "unit1",
				Objective: "Obj 1",
				Tasks:     []string{}, // empty tasks
			},
		},
	}
	analysisEmptyJSON, _ := json.Marshal(analysisEmpty)
	taskEmpty := &models.Task{
		ID:         "task-no-skip",
		Complexity: models.TaskComplexityMedium,
		Analysis:   analysisEmptyJSON,
	}

	logEmpty := &mockLogger{}
	stepEmpty := NewPlanStep(
		StepRuntime{Task: taskEmpty, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: taskEmpty},
		nil, nil, nil,
		&mockStatusUpdater{},
		logEmpty,
		&mockCheckpointLister{},
		8.0,
	)

	_, err = stepEmpty.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the log was NOT emitted
	for _, msg := range logEmpty.messages {
		if strings.Contains(msg, "Plan step skipped — execution units already provided by analyze step") {
			t.Errorf("did not expect skip log message, but got one")
		}
	}
}
