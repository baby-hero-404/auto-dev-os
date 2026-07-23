package steps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestParseReviewFindings_FieldMapping(t *testing.T) {
	parsed := map[string]any{
		"findings": []any{
			map[string]any{
				"repo":           "tool_zentao",
				"file":           "cmd/sync/main.go",
				"line":           float64(12),
				"severity":       "high",
				"recommendation": "fix it",
				"requires_fix":   true,
			},
		},
	}
	findings, err := ParseReviewFindings(parsed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Repo != "tool_zentao" || f.File != "cmd/sync/main.go" || f.Line != 12 ||
		f.Severity != "high" || f.Recommendation != "fix it" || !f.RequiresFix {
		t.Errorf("unexpected finding: %+v", f)
	}
}

func TestParseReviewVerdict_StructuredSchema(t *testing.T) {
	parsed := map[string]any{
		"spec_compliance": map[string]any{
			"verdict": "fail",
			"violations": []any{
				map[string]any{"requirement": "must validate input", "observed": "no validation found", "severity": "high"},
			},
		},
		"code_quality": map[string]any{
			"verdict": "pass",
		},
		"summary": "spec violation found",
	}
	v, ok := ParseReviewVerdict(parsed)
	if !ok {
		t.Fatalf("expected structured verdict to parse")
	}
	if v.SpecCompliance.Verdict != "fail" || v.CodeQuality.Verdict != "pass" || v.Summary != "spec violation found" {
		t.Errorf("unexpected verdict: %+v", v)
	}
	if len(v.SpecCompliance.Violations) != 1 || v.SpecCompliance.Violations[0].Requirement != "must validate input" {
		t.Errorf("unexpected violations: %+v", v.SpecCompliance.Violations)
	}
}

func TestParseReviewVerdict_FallbackWhenAbsent(t *testing.T) {
	parsed := map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "high"}}}
	_, ok := ParseReviewVerdict(parsed)
	if ok {
		t.Fatalf("expected ok=false for legacy single-verdict payload")
	}
}

func TestHasRepeatViolation(t *testing.T) {
	prevV := []models.SpecViolation{{Requirement: "must validate all user input fields"}}
	curSame := []models.SpecViolation{{Requirement: "must validate user input fields"}}
	curDifferent := []models.SpecViolation{{Requirement: "must add rate limiting to the login endpoint"}}

	if !hasRepeatViolation(prevV, curSame) {
		t.Errorf("expected fuzzy match (token overlap) to detect repeat violation")
	}
	if hasRepeatViolation(prevV, curDifferent) {
		t.Errorf("expected unrelated violation to not match")
	}
}

func TestReviewStep_SkipsEasyTask(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityEasy}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected skipped status, got: %v", result["status"])
	}
}

func TestReviewStep_SkipsViaPipelineConfigSkipWhenLabel(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Labels: []string{"hotfix"}}
	project := &models.Project{ID: "p1", PipelineConfig: []byte(`{"version":1,"pipeline":{"extends":"api_native","steps":[{"id":"review","skip_when":{"label":"hotfix"}}]}}`)}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: project},
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "skipped" {
		t.Errorf("expected skipped status via pipeline_config skip_when, got: %v", result["status"])
	}
}

func TestReviewStep_DoesNotSkipWhenLabelDoesNotMatch(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusFixing, Labels: []string{"backend"}}
	project := &models.Project{ID: "p1", PipelineConfig: []byte(`{"version":1,"pipeline":{"extends":"api_native","steps":[{"id":"review","skip_when":{"label":"hotfix"}}]}}`)}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: project},
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "bypassed_via_human_review" {
		t.Errorf("expected normal bypassed_via_human_review path (no skip), got: %v", result["status"])
	}
}

func TestReviewStep_BypassedViaHumanReview(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusFixing}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "bypassed_via_human_review" {
		t.Errorf("expected bypassed_via_human_review, got: %v", result["status"])
	}
}

func TestReviewStep_ErrWaitingApproval(t *testing.T) {
	task := &models.Task{ID: "t1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusHumanReview}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if !errors.Is(err, workflow.ErrWaitingApproval) {
		t.Errorf("expected ErrWaitingApproval, got: %v", err)
	}
}

func TestReviewStep_WithFindings(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	artifactMock := &mockArtifactSaver{}
	assigner := &mockReviewerAssigner{agent: &models.Agent{ID: "a2", Name: "Reviewer"}}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "high"}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		artifactMock,
		assigner,
		&mockCheckpointReader{count: 0},
		nil,
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !statusMock.called {
		t.Error("expected status updater to be called")
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected status to transition to fixing, got: %s", statusMock.lastStatus)
	}
	if !artifactMock.called {
		t.Error("expected artifact mock to be called")
	}
	if len(assigner.releasedIDs) != 1 {
		t.Fatalf("expected reviewer agent release, got %d releases", len(assigner.releasedIDs))
	}
	if assigner.releasedIDs[0] != "a2" {
		t.Fatalf("expected reviewer release of a2, got %s", assigner.releasedIDs[0])
	}
	if result["parsed"] == nil {
		t.Error("expected parsed result to be returned")
	}
}

// TestReviewStep_RequiresFixTrue regression-tests the legacy requires_fix:true boolean signal
// (independent of severity), lost when ParseReviewFindings/hasActionableFindings were rewritten
// to the typed models.ReviewFinding contract (Part 2 review finding).
func TestReviewStep_RequiresFixTrue(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "requires_fix": true}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected requires_fix:true (no severity) to route to fixing, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_ExceedsCycleLimit(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "high"}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1", MaxReviewFixCycles: 2}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 2}, // Already ran 2 cycles
		nil,
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusTesting {
		t.Errorf("expected cycle limit to redirect to testing, got: %s", statusMock.lastStatus)
	}
	if result["cycle_limit_reached"] != true {
		t.Error("expected cycle_limit_reached to be true in output")
	}
}

func TestReviewStep_PipelineConfigCycleLimitOverrideWins(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	findings := map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "high"}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{
			ID:                 "p1",
			MaxReviewFixCycles: 5,
			PipelineConfig:     []byte(`{"version":1,"policies":{"max_review_fix_cycles":1}}`),
		}},
		&mockLLMRunner{result: StepResult{"parsed": findings}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 1}, // Already ran 1 cycle; override limit is 1
		nil,
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["cycle_limit_reached"] != true {
		t.Error("expected pipeline_config max_review_fix_cycles override (1) to be enforced over project column (5)")
	}
}

func TestReviewStep_NoActionableFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings map[string]any
	}{
		{
			"missing severity",
			map[string]any{"findings": []any{map[string]any{"file": "main.go"}}},
		},
		{
			"low severity",
			map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "low"}}},
		},
		{
			"info severity",
			map[string]any{"findings": []any{map[string]any{"file": "main.go", "severity": "info"}}},
		},
		{
			"requires_fix false",
			map[string]any{"findings": []any{map[string]any{"file": "main.go", "requires_fix": false}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
			statusMock := &mockStatusUpdater{}
			step := NewReviewStep(
				StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
				&mockTaskReader{task: task},
				&mockProjectReader{project: &models.Project{ID: "p1"}},
				&mockLLMRunner{result: StepResult{"parsed": tc.findings}},
				&mockDiffCapturer{diffVal: "some diff"},
				&mockArtifactSaver{},
				nil,
				&mockCheckpointReader{count: 0},
				nil,
				statusMock,
				&mockLogger{},
			)
			_, err := step.Execute(context.Background(), workflow.StepContext{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if statusMock.lastStatus != models.TaskStatusTesting {
				t.Errorf("expected status to transition to testing for non-actionable, got: %s", statusMock.lastStatus)
			}
		})
	}
}

func TestReviewStep_SingleFindingFallback(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	// Top-level map acts as a single finding
	singleFinding := map[string]any{"file": "main.go", "severity": "high"}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": singleFinding}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected single finding fallback to transition to fixing, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_PRDiffFallbackUsesBackendWorktreeSuffix(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	diffMock := &mockDiffCapturer{}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleBackend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": map[string]any{"findings": []any{}}}},
		diffMock,
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		&mockStatusUpdater{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diffMock.lastWorkspaceSuffix != "-be-worktree" {
		t.Fatalf("expected backend worktree suffix fallback, got %q", diffMock.lastWorkspaceSuffix)
	}
}

func TestReviewStep_ObjectiveInjection(t *testing.T) {
	task := &models.Task{
		ID:         "t1",
		ProjectID:  "p1",
		Complexity: models.TaskComplexityMedium,
		Status:     models.TaskStatusReviewing,
		Analysis: []byte(`{
			"execution_units": [
				{"id": "u1", "objective": "Create database migration"},
				{"id": "u2", "objective": "Implement Repository layer"}
			]
		}`),
	}

	llmMock := &mockLLMRunner{result: StepResult{"parsed": map[string]any{"findings": []any{}}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleBackend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		llmMock,
		&mockDiffCapturer{},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		&mockStatusUpdater{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instr := llmMock.lastInstruction
	if !strings.Contains(instr, "EXECUTION OBJECTIVES") {
		t.Errorf("expected instruction to contain EXECUTION OBJECTIVES, got:\n%s", instr)
	}
	if !strings.Contains(instr, "- Node u1: Create database migration") {
		t.Errorf("expected instruction to contain u1 objective, got:\n%s", instr)
	}
	if !strings.Contains(instr, "- Node u2: Implement Repository layer") {
		t.Errorf("expected instruction to contain u2 objective, got:\n%s", instr)
	}
}

// TestReviewStep_ObjectiveInjection_LegacyNoExecutionUnits verifies REQ-003's stated
// fallback: a task with no execution_units at all (pre-FrozenContext / legacy task) must
// not gain an "EXECUTION OBJECTIVES" block — the addition is purely additive.
func TestReviewStep_ObjectiveInjection_LegacyNoExecutionUnits(t *testing.T) {
	task := &models.Task{
		ID:         "t-legacy",
		ProjectID:  "p1",
		Complexity: models.TaskComplexityMedium,
		Status:     models.TaskStatusReviewing,
		Analysis:   []byte(`{}`),
	}

	llmMock := &mockLLMRunner{result: StepResult{"parsed": map[string]any{"findings": []any{}}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleBackend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		llmMock,
		&mockDiffCapturer{},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		&mockStatusUpdater{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(llmMock.lastInstruction, "EXECUTION OBJECTIVES") {
		t.Errorf("expected no EXECUTION OBJECTIVES block for a task with no execution_units, got:\n%s", llmMock.lastInstruction)
	}
}

// TestReviewStep_ObjectiveInjection_UnitsWithoutObjectives covers the case where
// execution_units exist (FrozenContext present) but none carry a non-empty Objective —
// the block must still be omitted entirely, not rendered as an empty/near-empty section.
func TestReviewStep_ObjectiveInjection_UnitsWithoutObjectives(t *testing.T) {
	task := &models.Task{
		ID:         "t-empty-obj",
		ProjectID:  "p1",
		Complexity: models.TaskComplexityMedium,
		Status:     models.TaskStatusReviewing,
		Analysis: []byte(`{
			"execution_units": [
				{"id": "u1", "objective": ""}
			]
		}`),
	}

	llmMock := &mockLLMRunner{result: StepResult{"parsed": map[string]any{"findings": []any{}}}}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1", Role: models.AgentRoleBackend}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		llmMock,
		&mockDiffCapturer{},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		nil,
		&mockStatusUpdater{},
		&mockLogger{},
	)

	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(llmMock.lastInstruction, "EXECUTION OBJECTIVES") {
		t.Errorf("expected no EXECUTION OBJECTIVES block when no unit has a non-empty objective, got:\n%s", llmMock.lastInstruction)
	}
}

func newStructuredVerdictOut(specVerdict, qualityVerdict string) StepResult {
	parsed := map[string]any{
		"spec_compliance": map[string]any{"verdict": specVerdict},
		"code_quality":     map[string]any{"verdict": qualityVerdict},
		"summary":          "test",
	}
	return StepResult{"parsed": parsed}
}

func TestReviewStep_StructuredVerdict_BothPass_RoutesToTesting(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: newStructuredVerdictOut("pass", "pass")},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{},
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusTesting {
		t.Errorf("expected both-pass verdict to route to testing, got: %s", statusMock.lastStatus)
	}
	if result["review_verdict"] == nil {
		t.Errorf("expected review_verdict to be stored on the result for downstream fix instruction assembly")
	}
}

func TestReviewStep_StructuredVerdict_QualityOnlyFail_RoutesToFixing(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: newStructuredVerdictOut("pass", "fail")},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{},
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected quality-only fail to route to fixing, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_StructuredVerdict_SpecFail_FirstCycleRoutesToFixing(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	parsed := map[string]any{
		"spec_compliance": map[string]any{
			"verdict": "fail",
			"violations": []any{
				map[string]any{"requirement": "must validate all user input fields", "observed": "no validation"},
			},
		},
		"code_quality": map[string]any{"verdict": "pass"},
	}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": parsed}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{}, // no prior review checkpoints -> no escalation on first cycle
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error (should not escalate on first cycle): %v", err)
	}
	if statusMock.lastStatus != models.TaskStatusFixing {
		t.Errorf("expected first-cycle spec fail to route to fixing, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_SpecFailEscalation_PausesOnRepeatViolation(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}

	prevOutput := map[string]any{
		"review_verdict": map[string]any{
			"spec_compliance": map[string]any{
				"verdict": "fail",
				"violations": []any{
					map[string]any{"requirement": "must validate all user input fields", "observed": "no validation"},
				},
			},
		},
	}
	prevState := map[string]any{"status": workflow.StepStatusSuccess, "output": prevOutput}
	prevStateJSON, _ := json.Marshal(prevState)

	parsed := map[string]any{
		"spec_compliance": map[string]any{
			"verdict": "fail",
			"violations": []any{
				map[string]any{"requirement": "must validate user input fields", "observed": "still no validation"},
			},
		},
		"code_quality": map[string]any{"verdict": "pass"},
	}

	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1"}},
		&mockLLMRunner{result: StepResult{"parsed": parsed}},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 1},
		&mockCheckpointLister{cps: []models.WorkflowCheckpoint{
			{Step: workflow.StepReview, State: prevStateJSON},
		}},
		statusMock,
		&mockLogger{},
	)
	_, err := step.Execute(context.Background(), workflow.StepContext{})
	var pauseErr workflow.PauseError
	if !errors.As(err, &pauseErr) {
		t.Fatalf("expected PauseError for repeated spec violation, got: %v", err)
	}
	if !strings.Contains(pauseErr.Reason, "awaiting_review_escalation") {
		t.Errorf("expected pause reason to name the escalation state, got: %q", pauseErr.Reason)
	}
	if statusMock.lastStatus != models.TaskStatusHumanReview {
		t.Errorf("expected escalation to route task to human_review, got: %s", statusMock.lastStatus)
	}
}

func TestReviewStep_PolicySame_NoExclusionAndAdversarialDirective(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	llmMock := &mockLLMRunner{result: newStructuredVerdictOut("pass", "pass")}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1", ReviewHarnessPolicy: models.ReviewHarnessSame}},
		llmMock,
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{},
		statusMock,
		&mockLogger{},
	)
	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(llmMock.lastInstruction, "Adversarial audit mode") {
		t.Errorf("expected policy=same to always inject the adversarial audit directive, got:\n%s", llmMock.lastInstruction)
	}
}

func TestReviewStep_PipelineConfigReviewHarnessOverrideWins(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	llmMock := &mockLLMRunner{result: newStructuredVerdictOut("pass", "pass")}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{
			ID:                  "p1",
			ReviewHarnessPolicy: models.ReviewHarnessDifferentModel,
			PipelineConfig:      []byte(`{"version":1,"policies":{"review_harness":"same"}}`),
		}},
		llmMock,
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{},
		statusMock,
		&mockLogger{},
	)
	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(llmMock.lastInstruction, "Adversarial audit mode") {
		t.Errorf("expected pipeline_config review_harness override 'same' to win over project column 'different_model', got:\n%s", llmMock.lastInstruction)
	}
}

func TestReviewStep_DifferentModelPolicy_NoDirectiveByDefault(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	llmMock := &mockLLMRunner{result: newStructuredVerdictOut("pass", "pass")}
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1", ReviewHarnessPolicy: models.ReviewHarnessDifferentModel}},
		llmMock,
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{},
		statusMock,
		&mockLogger{},
	)
	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(llmMock.lastInstruction, "Adversarial audit mode") {
		t.Errorf("expected no adversarial directive when a genuinely different model was used, got:\n%s", llmMock.lastInstruction)
	}
}

func TestReviewStep_PriorSelfReviewFallback_InjectsAdversarialDirectiveNextCycle(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	llmMock := &mockLLMRunner{result: newStructuredVerdictOut("pass", "pass")}
	prevState := map[string]any{"status": workflow.StepStatusSuccess, "output": map[string]any{"self_review_fallback": true}}
	prevStateJSON, _ := json.Marshal(prevState)
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1", ReviewHarnessPolicy: models.ReviewHarnessDifferentModel}},
		llmMock,
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 1},
		&mockCheckpointLister{cps: []models.WorkflowCheckpoint{
			{Step: workflow.StepReview, State: prevStateJSON},
		}},
		statusMock,
		&mockLogger{},
	)
	if _, err := step.Execute(context.Background(), workflow.StepContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(llmMock.lastInstruction, "Adversarial audit mode") {
		t.Errorf("expected adversarial directive after a prior self-review fallback, got:\n%s", llmMock.lastInstruction)
	}
}

func TestReviewStep_CodedByAndReviewedByMetadata(t *testing.T) {
	task := &models.Task{ID: "t1", ProjectID: "p1", Complexity: models.TaskComplexityMedium, Status: models.TaskStatusReviewing}
	statusMock := &mockStatusUpdater{}
	out := newStructuredVerdictOut("pass", "pass")
	out["model"] = "gpt-5"
	out["provider"] = "openai"
	codeState := map[string]any{"status": workflow.StepStatusSuccess, "output": map[string]any{"model": "claude-x", "provider": "anthropic"}}
	codeStateJSON, _ := json.Marshal(codeState)
	step := NewReviewStep(
		StepRuntime{Task: task, Agent: &models.Agent{ID: "a1"}, JobID: "j1"},
		&mockTaskReader{task: task},
		&mockProjectReader{project: &models.Project{ID: "p1", ReviewHarnessPolicy: models.ReviewHarnessDifferentModel}},
		&mockLLMRunner{result: out},
		&mockDiffCapturer{diffVal: "some diff"},
		&mockArtifactSaver{},
		nil,
		&mockCheckpointReader{count: 0},
		&mockCheckpointLister{cps: []models.WorkflowCheckpoint{
			{Step: workflow.StepCodeBackend, State: codeStateJSON},
		}},
		statusMock,
		&mockLogger{},
	)
	result, err := step.Execute(context.Background(), workflow.StepContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	codedBy, ok := result["coded_by"].(map[string]any)
	if !ok || codedBy["model"] != "claude-x" || codedBy["provider"] != "anthropic" {
		t.Errorf("expected coded_by metadata from the code step, got: %v", result["coded_by"])
	}
	reviewedBy, ok := result["reviewed_by"].(map[string]any)
	if !ok || reviewedBy["model"] != "gpt-5" || reviewedBy["provider"] != "openai" {
		t.Errorf("expected reviewed_by metadata from the review LLM response, got: %v", result["reviewed_by"])
	}
}
