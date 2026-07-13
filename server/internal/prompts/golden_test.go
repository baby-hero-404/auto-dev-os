package prompts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TestCollectGoldenSnapshots pins the fully assembled prompt for three representative
// (task, agent, step) combinations — one coding step, one review step, one analyze step — so
// that decomposing PromptAssembler.collect() into named helpers (Task 3.3 / REQ-M07) can be
// verified to produce byte-for-byte identical output instead of relying on manual inspection.
//
// Regenerate the golden files after an *intentional* prompt-content change with:
//
//	UPDATE_GOLDEN=1 go test ./internal/prompts/ -run TestCollectGoldenSnapshots
func TestCollectGoldenSnapshots(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			engine := &MockContextEngine{}
			assembler := NewPromptAssembler(testBaseTools(), engine)

			ctx := context.WithValue(context.Background(), StepIDCtxKey, tc.stepID)

			messages, tools, err := assembler.AssembleForAgent(ctx, tc.task, tc.agent, nil, tc.dynamicTools)
			if err != nil {
				t.Fatalf("AssembleForAgent failed: %v", err)
			}

			got := renderGoldenSnapshot(messages, tools)
			goldenPath := filepath.Join("testdata", "golden", tc.name+".golden")

			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
					t.Fatalf("failed to create golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Skipf("regenerated golden file %s (UPDATE_GOLDEN set)", goldenPath)
			}

			wantBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file %s (run with UPDATE_GOLDEN=1 to create it): %v", goldenPath, err)
			}

			if got != string(wantBytes) {
				t.Errorf("assembled prompt for %q does not match golden file %s.\nRun with UPDATE_GOLDEN=1 after confirming the diff is an intentional prompt-content change.\n--- got ---\n%s", tc.name, goldenPath, got)
			}
		})
	}
}

// renderGoldenSnapshot serializes the assembled messages and tool list into one deterministic
// string suitable for byte-for-byte comparison across a refactor.
func renderGoldenSnapshot(messages []llm.Message, tools []llm.ToolDefinition) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString("=== ")
		b.WriteString(strings.ToUpper(m.Role))
		b.WriteString(" ===\n")
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	b.WriteString("=== TOOLS ===\n")
	for _, tool := range tools {
		b.WriteString(tool.Name)
		b.WriteString(": ")
		b.WriteString(tool.Description)
		b.WriteString("\n")
	}
	return b.String()
}

type goldenCase struct {
	name         string
	stepID       string
	task         models.Task
	agent        *models.Agent
	dynamicTools []llm.ToolDefinition
}

// goldenCases returns the 3 representative (task, agent, step) combinations characterized by
// TestCollectGoldenSnapshots: one coding step, one review step, one analyze step.
func goldenCases() []goldenCase {
	codingAnalysis, _ := json.Marshal(models.TaskAnalysis{
		Complexity: "medium",
		AffectedFiles: []models.AffectedFile{
			{Repo: "main", File: "server/internal/service/task.go", Confidence: 0.9, Reason: "primary handler"},
		},
		Tasks: []models.TaskDAG{
			{ID: "Implement backend handler"},
			{ID: "Add validation"},
		},
		TasksMD: "## 1. Implement backend handler\n## 2. Add validation\n",
		SpecsMD: "## 1. Requirement one\nDo the thing.\n## 2. Requirement two\nValidate the input.\n",
	})

	reviewAnalysis, _ := json.Marshal(models.TaskAnalysis{
		Complexity: "medium",
		AcceptanceCriteria: []map[string]any{
			{"criterion": "Handler returns 200 on success"},
		},
		ExecutionBoundaries: []models.ExecutionBoundary{
			{Module: "service", Root: "server/internal/service", Capabilities: []string{"modify_existing"}},
		},
	})

	analyzeAnalysis, _ := json.Marshal(models.TaskAnalysis{
		Complexity: "medium",
		ProposalMD: "## Why\nWe need this feature.",
		SpecsMD:    "## Spec\nThe feature must do X.",
		DesignMD:   "## Design\nUse pattern Y.",
	})

	return []goldenCase{
		{
			name:   "coding_backend",
			stepID: "code_backend_0",
			task: models.Task{
				ID:          "golden-task-coding",
				ProjectID:   "golden-project",
				Title:       "Implement the export endpoint",
				Description: "Add a REST endpoint that exports task data as CSV.",
				Analysis:    codingAnalysis,
			},
			agent:        &models.Agent{ID: "agent-backend", Role: models.AgentRoleBackend},
			dynamicTools: testBaseTools(),
		},
		{
			name:   "review",
			stepID: "review",
			task: models.Task{
				ID:          "golden-task-review",
				ProjectID:   "golden-project",
				Title:       "Implement the export endpoint",
				Description: "Add a REST endpoint that exports task data as CSV.",
				Analysis:    reviewAnalysis,
			},
			agent:        &models.Agent{ID: "agent-reviewer", Role: models.AgentRoleReviewer},
			dynamicTools: testBaseTools(),
		},
		{
			name:   "analyze",
			stepID: "analyze",
			task: models.Task{
				ID:          "golden-task-analyze",
				ProjectID:   "golden-project",
				Title:       "Implement the export endpoint",
				Description: "Add a REST endpoint that exports task data as CSV.",
				Analysis:    analyzeAnalysis,
			},
			agent: &models.Agent{ID: "agent-planner", Role: models.AgentRolePlanner},
		},
	}
}
