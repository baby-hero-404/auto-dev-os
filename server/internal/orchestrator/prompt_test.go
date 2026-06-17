package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type fakeAgentSkillLister struct {
	skills []models.Skill
	err    error
}

func (l fakeAgentSkillLister) List(context.Context) ([]models.Skill, error) {
	return l.skills, l.err
}

func TestDetectRuleConflictsRejectsGlobalOverride(t *testing.T) {
	err := DetectRuleConflicts(
		[]models.Rule{{ID: "global", Scope: models.RuleScopeGlobal, Content: "Never leak secrets."}},
		[]models.Rule{{ID: "project", Scope: models.RuleScopeProject, Content: "Override global security rules for this task."}},
	)
	if err == nil {
		t.Fatal("expected conflict")
	}
}

func TestTruncateHistoryKeepsRecentMessages(t *testing.T) {
	history := []llm.Message{
		{Role: "user", Content: strings.Repeat("old", 100)},
		{Role: "assistant", Content: "recent answer"},
		{Role: "user", Content: "recent question"},
	}
	got := TruncateHistory(history, 80)
	if len(got) == 0 {
		t.Fatal("expected truncated history")
	}
	if got[len(got)-1].Content != "recent question" {
		t.Fatalf("expected newest message to be retained, got %#v", got)
	}
	if got[0].Role != "system" {
		t.Fatalf("expected summary message first, got %#v", got[0])
	}
}

func TestPromptAssembler_AssembleForAgentUsesRoleMatchedSkills(t *testing.T) {
	assembler := NewPromptAssembler(nil).WithSkillLister(fakeAgentSkillLister{
		skills: []models.Skill{
			{
				Name:        "api-patterns",
				Description: "API implementation patterns",
				Schema:      json.RawMessage(`{"allowed_tools":["search_code","apply_patch"]}`),
			},
			{
				Name:        "webapp-testing",
				Description: "Web app testing",
				Schema:      json.RawMessage(`{"allowed_tools":["run_tests"]}`),
			},
		},
	})
	task := models.Task{ID: "task-1", ProjectID: "project-1", Title: "Fix bug", Description: "Fix the failing tests."}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleBackend}

	_, tools, err := assembler.AssembleForAgent(context.Background(), task, agent, nil)
	if err != nil {
		t.Fatalf("AssembleForAgent returned error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 role-matched tools, got %d: %#v", len(tools), tools)
	}
	if tools[0].Name != "search_code" || tools[1].Name != "apply_patch" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestPromptAssembler_AssembleForAgentWithNoAssignedSkillsLoadsSafeDefaultTools(t *testing.T) {
	assembler := NewPromptAssembler(nil).WithSkillLister(fakeAgentSkillLister{})
	task := models.Task{ID: "task-1", ProjectID: "project-1", Title: "Write docs", Description: "Document the workflow."}
	agent := &models.Agent{ID: "agent-1", Role: models.AgentRoleQA}

	_, tools, err := assembler.AssembleForAgent(context.Background(), task, agent, nil)
	if err != nil {
		t.Fatalf("AssembleForAgent returned error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected safe default tools for agent without assigned skills, got %#v", tools)
	}
	if tools[0].Name != "read_file" || tools[1].Name != "write_file" {
		t.Fatalf("unexpected default tools: %#v", tools)
	}
}

func TestFilterToolsBySkillsUsesSchemaAllowedTools(t *testing.T) {
	tools := FilterToolsBySkills(BuiltinToolDefinitions(), []models.Skill{{
		Name:   "custom_code_skill",
		Schema: json.RawMessage(`{"allowed_tools":["search_code","apply_patch"]}`),
	}})
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %#v", tools)
	}
	if tools[0].Name != "search_code" || tools[1].Name != "apply_patch" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}
