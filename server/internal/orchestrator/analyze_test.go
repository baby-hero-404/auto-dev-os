package orchestrator

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func TestOrchestrator_StepAnalyze(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-tools-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name         string
		task         *models.Task
		agent        *models.Agent
		llmResponses map[string]string
		llmQueue     []*llm.Response
		runtimeOut   map[string]string
		assert       func(t *testing.T, task *models.Task, res map[string]any, err error, mockLLM *mockLLMProvider, runtime *mockAnalyzeSandboxRuntime)
	}{
		{
			name: "AutonomyLevel_AutoApproves",
			task: &models.Task{
				ID:          "task-1",
				ProjectID:   "proj-1",
				Title:       "Test Task",
				Description: "Simple test description",
				Complexity:  models.TaskComplexityMedium,
				SpecStatus:  models.TaskSpecStatusDraft,
			},
			agent: &models.Agent{
				ID:            "agent-1",
				Name:          "Agent 1",
				AutonomyLevel: models.AgentAutonomyAutonomous,
			},
			llmResponses: map[string]string{
				"": `{"complexity":"medium","primary_category":"backend","spec_status":"approved","clarification_questions":[],"affected_files":[],"execution_phases":[],"system_prompt":"mock","proposal_md":"## Proposal","specs_md":"## Specs","execution_boundaries":{"allowed":["."]},"acceptance_criteria":[{"id":"AC-1","expected":"ok"}]}`,
			},
			assert: func(t *testing.T, task *models.Task, res map[string]any, err error, mockLLM *mockLLMProvider, runtime *mockAnalyzeSandboxRuntime) {
				if err != nil {
					t.Fatalf("unexpected error for autonomous agent: %v", err)
				}
				if task.SpecStatus != models.TaskSpecStatusAutoApproved {
					t.Errorf("expected spec status AutoApproved, got %s", task.SpecStatus)
				}
				if task.Status != models.TaskStatusCoding {
					t.Errorf("expected task status Coding, got %s", task.Status)
				}
				if res["spec_status"] != models.TaskSpecStatusAutoApproved {
					t.Errorf("expected output spec_status AutoApproved, got %v", res["spec_status"])
				}
			},
		},
		{
			name: "UsesNativeToolCalls",
			task: &models.Task{
				ID:         "task-native",
				ProjectID:  "proj-native",
				Title:      "Inspect code",
				Status:     models.TaskStatusAnalyzing,
				Complexity: models.TaskComplexityMedium,
				SpecStatus: models.TaskSpecStatusDraft,
			},
			agent: &models.Agent{
				ID:            "agent-native",
				AutonomyLevel: models.AgentAutonomyAutonomous,
			},
			llmQueue: []*llm.Response{
				{
					Model: "mock-model",
					ToolCalls: []llm.ToolCall{{
						ID:        "call-list-files",
						Name:      "list_files",
						Arguments: "{}",
					}},
				},
				{
					Model:   "mock-model",
					Content: `{"complexity":"medium","primary_category":"backend","scope":"test","affected_files":["src/main.go"],"execution_phases":[{"phase":"1","tasks":["t"]}],"clarification_questions":[],"required_skills":[],"proposal_md":"## P","specs_md":"## S","design_md":"## D","tasks_md":"## T","acceptance_criteria":[{"id":"AC-1","expected":"ok"}],"execution_boundaries":{"allowed":["src/"]}}`,
				},
			},
			runtimeOut: map[string]string{"find .": "src/main.go\n"},
			assert: func(t *testing.T, task *models.Task, res map[string]any, err error, mockLLM *mockLLMProvider, runtime *mockAnalyzeSandboxRuntime) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(mockLLM.lastChatOptions.Tools) != 3 {
					t.Fatalf("expected analyze tool definitions, got %+v", mockLLM.lastChatOptions.Tools)
				}
				if len(runtime.commands) == 0 || !strings.Contains(strings.Join(runtime.commands, "\n"), "find .") {
					t.Fatalf("expected list_files native tool to execute, got %#v", runtime.commands)
				}
			},
		},
		{
			name: "FallbackForcesReview",
			task: &models.Task{
				ID:         "task-fallback",
				ProjectID:  "proj-fallback",
				Status:     models.TaskStatusAnalyzing,
				Complexity: models.TaskComplexityMedium,
				SpecStatus: models.TaskSpecStatusDraft,
			},
			agent: &models.Agent{
				ID:            "agent-fallback",
				AutonomyLevel: models.AgentAutonomyAutonomous,
			},
			llmResponses: map[string]string{
				"": "this is not valid JSON and will fail parsing",
			},
			assert: func(t *testing.T, task *models.Task, res map[string]any, err error, mockLLM *mockLLMProvider, runtime *mockAnalyzeSandboxRuntime) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "workflow paused for human spec review due to fallback from malformed analyzer output") {
					t.Fatalf("expected pause error, got %v", err)
				}
				if task.SpecStatus != models.TaskSpecStatusPendingReview {
					t.Errorf("expected spec status PendingReview, got %s", task.SpecStatus)
				}
				if task.Status != models.TaskStatusSpecReview {
					t.Errorf("expected task status SpecReview, got %s", task.Status)
				}
			},
		},
		{
			name: "ComplexityChangeTriggersGraphRebuild",
			task: &models.Task{
				ID:         "task-graph-change",
				ProjectID:  "proj-graph-change",
				Status:     models.TaskStatusAnalyzing,
				Complexity: models.TaskComplexityEasy,
				SpecStatus: models.TaskSpecStatusDraft,
			},
			agent: &models.Agent{
				ID:            "agent-graph-change",
				AutonomyLevel: models.AgentAutonomyAutonomous,
			},
			llmResponses: map[string]string{
				"": `{"complexity":"hard","primary_category":"backend","scope":"test","affected_files":[],"risks":[],"risk_domains":[],"execution_phases":[],"clarification_questions":[],"required_skills":[],"proposal_md":"## P","specs_md":"## S","acceptance_criteria":[{"id":"AC-1","expected":"ok"}],"execution_boundaries":{"allowed":["src/"]}}`,
			},
			assert: func(t *testing.T, task *models.Task, res map[string]any, err error, mockLLM *mockLLMProvider, runtime *mockAnalyzeSandboxRuntime) {
				if err == nil {
					t.Fatalf("expected ErrGraphChanged, got nil")
				}
				if !errors.Is(err, workflow.ErrGraphChanged) {
					t.Fatalf("expected workflow.ErrGraphChanged, got %v", err)
				}
				if task.Complexity != models.TaskComplexityHard {
					t.Errorf("expected complexity to update to hard, got %s", task.Complexity)
				}
				if task.SpecStatus != models.TaskSpecStatusAutoApproved {
					t.Errorf("expected auto_approved spec status, got %s", task.SpecStatus)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taskRepo := &mockTaskRepo{task: tc.task}
			workflowRepo := &mockWorkflowRepo{job: &models.WorkflowJob{ID: "job-" + tc.task.ID}}
			mockLLM := &mockLLMProvider{responses: tc.llmResponses, responseQueue: tc.llmQueue}
			runtime := &mockAnalyzeSandboxRuntime{outputs: tc.runtimeOut}

			orch := New(taskRepo, workflowRepo, nil, runtime,
				WithWorkspaceRoot(tmpDir),
				WithLLMProvider(mockLLM),
			)

			runners := orch.stepRunners(tc.task, tc.agent, "job-"+tc.task.ID, "")
			analyzeRunner := runners[workflow.StepAnalyze]

			res, err := analyzeRunner(context.Background(), workflow.StepContext{})
			tc.assert(t, tc.task, res, err, mockLLM, runtime)
		})
	}
}
