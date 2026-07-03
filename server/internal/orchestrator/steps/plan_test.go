package steps

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestShouldSkipFrontend_BackendOnly(t *testing.T) {
	analysis := models.TaskAnalysis{PrimaryCategory: "backend"}
	subtasks := map[string][]string{"backend": {"task1"}}
	if !shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=true for backend-only category")
	}
}

func TestShouldSkipFrontend_DatabaseCategory(t *testing.T) {
	analysis := models.TaskAnalysis{PrimaryCategory: "database"}
	subtasks := map[string][]string{}
	if !shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=true for database category")
	}
}

func TestShouldSkipFrontend_HasFrontendTasks(t *testing.T) {
	analysis := models.TaskAnalysis{PrimaryCategory: "frontend"}
	subtasks := map[string][]string{
		"backend":  {"be task"},
		"frontend": {"fe task"},
	}
	if shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=false when frontend subtasks exist")
	}
}

func TestShouldSkipFrontend_FrontendTasksOverrideBackendCategory(t *testing.T) {
	analysis := models.TaskAnalysis{PrimaryCategory: "backend"}
	subtasks := map[string][]string{
		"backend":  {"be task"},
		"frontend": {"fe task"},
	}
	if shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=false when frontend subtasks exist even if category is backend")
	}
}

func TestShouldSkipFrontend_BackendCategory_WithSubtasks_KeepsFrontend(t *testing.T) {
	analysis := models.TaskAnalysis{PrimaryCategory: "backend"}
	subtasks := map[string][]string{
		"frontend": {"fe task"},
	}
	if shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=false when frontend subtasks exist")
	}
}

func TestShouldSkipFrontend_NoSubtasksButFrontendFiles(t *testing.T) {
	analysis := models.TaskAnalysis{
		PrimaryCategory: "",
		AffectedFiles:   []string{"web/src/App.tsx", "server/main.go"},
	}
	subtasks := map[string][]string{"backend": {"be task"}}
	if shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=false when affected_files contains frontend files")
	}
}

func TestShouldSkipFrontend_NoSubtasksNoFiles(t *testing.T) {
	analysis := models.TaskAnalysis{
		PrimaryCategory: "",
		AffectedFiles:   []string{"server/main.go", "internal/repo.go"},
	}
	subtasks := map[string][]string{"backend": {"be task"}}
	if !shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=true when no frontend subtasks or files")
	}
}

func TestShouldSkipFrontend_UnknownCategory_NoSubtasks_Skips(t *testing.T) {
	analysis := models.TaskAnalysis{
		PrimaryCategory: "mobile",
		AffectedFiles:   []string{"server/main.go"},
	}
	subtasks := map[string][]string{}
	if !shouldSkipFrontend(analysis, subtasks) {
		t.Error("expected skip=true when category is unknown and no frontend signals exist")
	}
}

func TestPlanStep_NoLLMCall(t *testing.T) {
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		TasksMD:         "## Backend\n- [ ] Task 1\n",
		ExecutionPlan:   []string{"step1", "step2"},
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
	analysis := models.TaskAnalysis{Complexity: "medium", TasksMD: "## Backend\n- [ ] t1\n"}
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
		TasksMD:         "## Backend\n- [ ] be1\n## UI Components\n- [ ] fe1\n- [ ] fe2\n",
		ExecutionPlan:   []string{"plan step 1"},
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

	// Check execution_plan propagated
	plan, ok := result["execution_plan"].([]string)
	if !ok {
		t.Fatalf("execution_plan wrong type: %T", result["execution_plan"])
	}
	if len(plan) != 1 || plan[0] != "plan step 1" {
		t.Errorf("unexpected execution_plan: %v", plan)
	}
}

func TestPlanStep_EmitsLogs(t *testing.T) {
	analysis := models.TaskAnalysis{
		Complexity:      "medium",
		PrimaryCategory: "backend",
		TasksMD:         "## Backend\n- [ ] t1\n",
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
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"parsing tasks_md",
		"backend subtasks",
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
