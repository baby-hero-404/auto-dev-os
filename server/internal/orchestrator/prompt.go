package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/retrieval"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type PromptAssembler struct {
	retriever retrieval.ContextRetriever
	rules     *repository.RuleRepo
	skills    AgentSkillLister
	root      string
}

type AgentSkillLister interface {
	ListByAgentID(context.Context, string) ([]models.Skill, error)
}

func NewPromptAssembler(retriever retrieval.ContextRetriever) *PromptAssembler {
	return &PromptAssembler{retriever: retriever, root: defaultPromptRoot()}
}

func NewPromptAssemblerWithRules(retriever retrieval.ContextRetriever, rules *repository.RuleRepo, root string) *PromptAssembler {
	if root == "" {
		root = defaultPromptRoot()
	}
	return &PromptAssembler{retriever: retriever, rules: rules, root: root}
}

func (a *PromptAssembler) WithSkillLister(skills AgentSkillLister) *PromptAssembler {
	a.skills = skills
	return a
}

func (a *PromptAssembler) Assemble(ctx context.Context, task models.Task) ([]llm.Message, []ToolDefinition, error) {
	return a.AssembleForAgent(ctx, task, nil, nil)
}

type contextKey string

const memoriesCtxKey contextKey = "retrieved_memories"

func (a *PromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []ToolDefinition, error) {
	var contextBlock string
	if a != nil && a.retriever != nil {
		snippets, err := a.retriever.RetrieveContext(ctx, task.Title+"\n"+task.Description, 8)
		if err != nil {
			return nil, nil, err
		}
		contextBlock = formatContextSnippets(snippets)
	}

	system, err := a.systemPrompt(ctx, task, agent)
	if err != nil {
		return nil, nil, err
	}
	user := "Task: " + task.Title + "\n\n" + task.Description
	if contextBlock != "" {
		user += "\n\nContext:\n" + contextBlock
	}
	if memories, ok := ctx.Value(memoriesCtxKey).([]models.EpisodicMemory); ok && len(memories) > 0 {
		user += "\n\nRetrieved Memories:\n" + formatMemories(memories)
	}
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	messages = append(messages, TruncateHistory(history, 12000)...)
	tools, err := a.toolDefinitionsForAgent(ctx, agent)
	if err != nil {
		return nil, nil, err
	}
	return messages, tools, nil
}

func (a *PromptAssembler) toolDefinitionsForAgent(ctx context.Context, agent *models.Agent) ([]ToolDefinition, error) {
	if agent == nil || a == nil || a.skills == nil {
		return BuiltinToolDefinitions(), nil
	}
	skills, err := a.skills.ListByAgentID(ctx, agent.ID)
	if err != nil {
		return nil, err
	}
	return ToolDefinitionsFromSkills(skills), nil
}

func ToolDefinitionsFromSkills(skills []models.Skill) []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(skills))
	for _, skill := range skills {
		if strings.TrimSpace(skill.Name) == "" {
			continue
		}
		schema := skill.Schema
		if len(schema) == 0 || !json.Valid(schema) {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		tools = append(tools, ToolDefinition{
			Name:        skill.Name,
			Description: skill.Description,
			Parameters:  schema,
		})
	}
	return tools
}

func formatMemories(memories []models.EpisodicMemory) string {
	var b strings.Builder
	for _, mem := range memories {
		b.WriteString(fmt.Sprintf("[%s/%s] %s\n", mem.Tier, mem.Category, mem.Summary))
		if mem.Content != "" && mem.Content != mem.Summary {
			b.WriteString(fmt.Sprintf("Detail: %s\n", mem.Content))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func formatContextSnippets(snippets []models.ContextSnippet) string {
	var b strings.Builder
	for _, snippet := range snippets {
		b.WriteString("```")
		b.WriteString(snippet.Path)
		b.WriteString("\n")
		b.WriteString(snippet.Content)
		if !strings.HasSuffix(snippet.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}
	return b.String()
}

func (a *PromptAssembler) systemPrompt(ctx context.Context, task models.Task, agent *models.Agent) (string, error) {
	root := defaultPromptRoot()
	if a != nil && a.root != "" {
		root = a.root
	}
	parts := []string{}
	if content, err := readOptional(filepath.Join(root, "core", "system_prompt.md")); err == nil && strings.TrimSpace(content) != "" {
		parts = append(parts, "# Base System Prompt\n"+content)
	}
	if agent != nil {
		if content, err := readOptional(filepath.Join(root, "antigravity", "agents", personaFile(agent.Role))); err == nil && strings.TrimSpace(content) != "" {
			parts = append(parts, "# Agent Persona\n"+content)
		}
	}

	globalRules, projectRules, err := a.loadRules(ctx, task.ProjectID)
	if err != nil {
		return "", err
	}
	if err := DetectRuleConflicts(globalRules, projectRules); err != nil {
		return "", err
	}
	if len(globalRules) > 0 {
		parts = append(parts, "# Global Rules\n"+formatRules(globalRules))
	}
	if len(projectRules) > 0 {
		parts = append(parts, "# Project Rules\n"+formatRules(projectRules))
	}
	parts = append(parts, `# Execution Rules
- Prefer apply_patch for source edits instead of rewriting full files.
- Run tests through run_tests when a change is executable.
- Return structured JSON when the workflow step requests JSON output.`)
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

func (a *PromptAssembler) loadRules(ctx context.Context, projectID string) ([]models.Rule, []models.Rule, error) {
	if a == nil || a.rules == nil {
		return nil, nil, nil
	}
	rules, err := a.rules.ListByProjectID(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	globalRules := []models.Rule{}
	projectRules := []models.Rule{}
	for _, rule := range rules {
		switch rule.Scope {
		case models.RuleScopeGlobal:
			globalRules = append(globalRules, rule)
		default:
			projectRules = append(projectRules, rule)
		}
	}
	return globalRules, projectRules, nil
}

func DetectRuleConflicts(globalRules, projectRules []models.Rule) error {
	if len(globalRules) == 0 || len(projectRules) == 0 {
		return nil
	}
	conflictPattern := regexp.MustCompile(`(?i)\b(ignore|override|disable|bypass)\b.*\b(global|strict|security|rule)`)
	for _, rule := range projectRules {
		if conflictPattern.MatchString(rule.Content) {
			return fmt.Errorf("project rule %s conflicts with global governance rules", rule.ID)
		}
	}
	return nil
}

func TruncateHistory(history []llm.Message, maxChars int) []llm.Message {
	if maxChars <= 0 || len(history) == 0 {
		return nil
	}
	selected := []llm.Message{}
	total := 0
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		size := len(msg.Role) + len(msg.Content)
		if total+size > maxChars {
			selected = append(selected, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("Earlier conversation summarized: %d messages omitted to stay within token budget.", i+1),
			})
			break
		}
		total += size
		selected = append(selected, msg)
	}
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	return selected
}

func formatRules(rules []models.Rule) string {
	lines := make([]string, 0, len(rules))
	for _, rule := range rules {
		lines = append(lines, fmt.Sprintf("- [%s/%s] %s", rule.Scope, rule.Enforcement, strings.TrimSpace(rule.Content)))
	}
	return strings.Join(lines, "\n")
}

func personaFile(role string) string {
	switch strings.ToLower(role) {
	case models.AgentRolePlanner:
		return "project-planner.md"
	case models.AgentRoleFrontend:
		return "frontend-specialist.md"
	case models.AgentRoleReviewer:
		return "security-auditor.md"
	case models.AgentRoleQA:
		return "test-engineer.md"
	default:
		return "backend-specialist.md"
	}
}

func readOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func defaultPromptRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Clean(filepath.Join("..", "resources", "prompt_base"))
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "resources", "prompt_base"))
}
