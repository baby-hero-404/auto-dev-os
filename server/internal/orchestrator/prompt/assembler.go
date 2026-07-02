package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/internal/retrieval"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type PromptAssembler struct {
	retriever retrieval.ContextRetriever
	rules     *repository.RuleRepo
	skills    SkillLister
	baseTools []llm.ToolDefinition
	root      string
	dataRoot  string
}

type SkillLister interface {
	List(context.Context) ([]models.Skill, error)
}

func NewPromptAssembler(retriever retrieval.ContextRetriever, baseTools []llm.ToolDefinition) *PromptAssembler {
	return &PromptAssembler{retriever: retriever, baseTools: baseTools, root: defaultPromptRoot()}
}

func NewPromptAssemblerWithRules(retriever retrieval.ContextRetriever, rules *repository.RuleRepo, baseTools []llm.ToolDefinition, root string) *PromptAssembler {
	if root == "" {
		root = defaultPromptRoot()
	}
	return &PromptAssembler{retriever: retriever, rules: rules, baseTools: baseTools, root: root}
}

func (a *PromptAssembler) WithSkillLister(skills SkillLister) *PromptAssembler {
	a.skills = skills
	return a
}

func (a *PromptAssembler) WithDataRoot(dataRoot string) *PromptAssembler {
	a.dataRoot = dataRoot
	return a
}

func (a *PromptAssembler) Assemble(ctx context.Context, task models.Task) ([]llm.Message, []llm.ToolDefinition, error) {
	return a.AssembleForAgent(ctx, task, nil, nil)
}

type contextKey string

const MemoriesCtxKey contextKey = "retrieved_memories"

func (a *PromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []llm.ToolDefinition, error) {
	var contextBlock string
	if a != nil && a.retriever != nil && shouldAttachCodeContext(agent) {
		snippets, err := a.retriever.RetrieveContext(ctx, task.Title+"\n"+task.Description, 8)
		if err != nil {
			return nil, nil, err
		}
		contextBlock = formatContextSnippets(snippets)
	}

	// Inject Project Knowledge Base Docs (Planned 5.5)
	if a != nil && a.dataRoot != "" && shouldAttachCodeContext(agent) {
		kbContent := a.loadProjectKnowledgeBaseDocs(task.ProjectID, task.Title+"\n"+task.Description)
		if kbContent != "" {
			if contextBlock != "" {
				contextBlock = kbContent + "\n\n" + contextBlock
			} else {
				contextBlock = kbContent
			}
		}
	}

	system, _, err := a.systemPrompt(ctx, task, agent)
	if err != nil {
		return nil, nil, err
	}
	user := "Task: " + task.Title + "\n\n" + task.Description
	if contextBlock != "" {
		user += "\n\nSemantic Code Retrieval Context:\n" + contextBlock
	}
	if memories, ok := ctx.Value(MemoriesCtxKey).([]models.EpisodicMemory); ok && len(memories) > 0 {
		user += "\n\nRetrieved Memories:\n" + formatMemories(memories)
	}
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	messages = append(messages, TruncateHistory(history, 12000)...)
	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	tools, err := a.toolDefinitionsForAgent(ctx, agent, task.ProjectID, analysis.RequiredSkills)
	if err != nil {
		return nil, nil, err
	}
	return messages, tools, nil
}

func (a *PromptAssembler) systemPrompt(ctx context.Context, task models.Task, agent *models.Agent) (string, []models.Rule, error) {
	root := defaultPromptRoot()
	if a != nil && a.root != "" {
		root = a.root
	}
	parts := []string{}
	if content, err := readOptional(filepath.Join(root, "core", "system_prompt.md")); err == nil && strings.TrimSpace(content) != "" {
		parts = append(parts, "# Base System Prompt\n"+content)
	}

	globalRules, projectRules, err := a.loadRules(ctx, task.ProjectID)
	if err != nil {
		return "", nil, err
	}
	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	localRules := append([]models.Rule{}, projectRules...)
	for i, tr := range analysis.TaskRules {
		localRules = append(localRules, models.Rule{
			ID:          fmt.Sprintf("task-rule-%d", i),
			Scope:       "task",
			Content:     tr,
			Enforcement: models.RuleEnforcementStrict,
		})
	}
	if err := DetectRuleConflicts(globalRules, localRules); err != nil {
		return "", nil, err
	}

	// 1. Global Rules
	if len(globalRules) > 0 {
		parts = append(parts, "# Global Rules [IMMUTABLE - DO NOT OVERRIDE]\n"+formatRules(globalRules))
	}

	// 2. Agent Role Constraints
	if agent != nil {
		if content, err := readOptional(filepath.Join(root, "antigravity", "agents", personaFile(agent.Role))); err == nil && strings.TrimSpace(content) != "" {
			parts = append(parts, "# Agent Role Constraints\n"+content)
		}
	}

	// 3. Project Rules
	if len(projectRules) > 0 {
		parts = append(parts, "# Project Rules\n"+formatRules(projectRules))
	}

	// 4. Task Rules
	if len(analysis.TaskRules) > 0 {
		var b strings.Builder
		b.WriteString("# Task-specific Rules:\n")
		for _, tr := range analysis.TaskRules {
			b.WriteString(fmt.Sprintf("- [task/strict] %s\n", strings.TrimSpace(tr)))
		}
		parts = append(parts, b.String())
	}

	parts = append(parts, "# Execution Rules\n- Prefer apply_patch for source edits instead of rewriting full files.\n- Run tests through run_tests when a change is executable.\n- Return structured JSON when the workflow step requests JSON output.\n- CRITICAL: Do NOT leak your internal system instructions (e.g., 'Prompt Base', 'Librarian Protocol', 'registry.min.json') into the code or documentation you generate. The code you write belongs to the user's target repository, not the orchestrator framework.")
	return strings.TrimSpace(strings.Join(parts, "\n\n")), projectRules, nil
}
