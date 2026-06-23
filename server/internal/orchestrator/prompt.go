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
	skills    SkillLister
	root      string
	dataRoot  string
}

type SkillLister interface {
	List(context.Context) ([]models.Skill, error)
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

func (a *PromptAssembler) WithSkillLister(skills SkillLister) *PromptAssembler {
	a.skills = skills
	return a
}

func (a *PromptAssembler) WithDataRoot(dataRoot string) *PromptAssembler {
	a.dataRoot = dataRoot
	return a
}

func (a *PromptAssembler) Assemble(ctx context.Context, task models.Task) ([]llm.Message, []ToolDefinition, error) {
	return a.AssembleForAgent(ctx, task, nil, nil)
}

type contextKey string

const memoriesCtxKey contextKey = "retrieved_memories"

func (a *PromptAssembler) AssembleForAgent(ctx context.Context, task models.Task, agent *models.Agent, history []llm.Message) ([]llm.Message, []ToolDefinition, error) {
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
	if memories, ok := ctx.Value(memoriesCtxKey).([]models.EpisodicMemory); ok && len(memories) > 0 {
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

func shouldAttachCodeContext(agent *models.Agent) bool {
	return true
}

func (a *PromptAssembler) toolDefinitionsForAgent(ctx context.Context, agent *models.Agent, projectID string, requiredSkills []string) ([]ToolDefinition, error) {
	if agent == nil || a == nil {
		return BuiltinToolDefinitions(), nil
	}
	var allSkills []models.Skill
	if a.skills != nil {
		var err error
		allSkills, err = a.skills.List(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Merge with project-specific disk skills
	diskSkills, err := a.loadProjectDiskSkills(projectID)
	if err == nil && len(diskSkills) > 0 {
		skillMap := make(map[string]models.Skill)
		for _, s := range allSkills {
			skillMap[strings.ToLower(s.Name)] = s
		}
		for _, s := range diskSkills {
			skillMap[strings.ToLower(s.Name)] = s
		}
		allSkills = make([]models.Skill, 0, len(skillMap))
		for _, s := range skillMap {
			allSkills = append(allSkills, s)
		}
	}

	requiredMap := make(map[string]bool)
	for _, req := range requiredSkills {
		requiredMap[strings.ToLower(strings.TrimSpace(req))] = true
	}

	skills := make([]models.Skill, 0, len(allSkills))
	for _, skill := range allSkills {
		if isSkillMatchingRole(skill.Name, agent.Role) || requiredMap[strings.ToLower(skill.Name)] {
			skills = append(skills, skill)
		}
	}
	return FilterToolsBySkills(BuiltinToolDefinitions(), skills), nil
}

func isSkillMatchingRole(skillName string, agentRole string) bool {
	agentRole = strings.ToLower(agentRole)
	skillName = strings.ToLower(skillName)

	// Always assign clean-code to developer/reviewer agents
	if skillName == "clean-code" && (agentRole == "backend" || agentRole == "frontend" || agentRole == "reviewer") {
		return true
	}

	switch agentRole {
	case "backend", "backend-specialist":
		backendSkills := map[string]bool{
			"bash-linux":            true,
			"golang-best-practices": true,
			"nodejs-best-practices": true,
			"python-patterns":       true,
			"database-design":       true,
			"api-patterns":          true,
			"systematic-debugging":  true,
			"clean-code":            true,
		}
		return backendSkills[skillName]
	case "frontend", "frontend-specialist":
		frontendSkills := map[string]bool{
			"react-patterns":        true,
			"nextjs-best-practices": true,
			"tailwind-patterns":     true,
			"ux-ui-pro-max":         true,
			"frontend-design":       true,
			"clean-code":            true,
		}
		return frontendSkills[skillName]
	case "reviewer":
		reviewerSkills := map[string]bool{
			"code-review-checklist": true,
			"review-pre-commit-git": true,
			"clean-code":            true,
		}
		return reviewerSkills[skillName]
	case "qa", "test-engineer":
		qaSkills := map[string]bool{
			"testing-patterns":               true,
			"tdd-workflow":                   true,
			"webapp-testing":                 true,
			"verification-before-completion": true,
			"clean-code":                     true,
		}
		return qaSkills[skillName]
	case "security-auditor":
		securitySkills := map[string]bool{
			"red-teaming":           true,
			"vulnerability-scanner": true,
			"red-team-tactics":      true,
		}
		return securitySkills[skillName]
	case "planner", "project-planner":
		plannerSkills := map[string]bool{
			"architecture":   true,
			"plan-writing":   true,
			"project-memory": true,
			"brainstorming":  true,
		}
		return plannerSkills[skillName]
	}
	return false
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

func FilterToolsBySkills(tools []ToolDefinition, skills []models.Skill) []ToolDefinition {
	allowed := allowedToolSetFromSkills(skills)
	if len(allowed) == 0 {
		allowed = map[string]bool{"read_file": true, "write_file": true}
	}
	filtered := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	if len(filtered) == 0 {
		for _, tool := range tools {
			if tool.Name == "read_file" || tool.Name == "write_file" {
				filtered = append(filtered, tool)
			}
		}
	}
	return filtered
}

func allowedToolSetFromSkills(skills []models.Skill) map[string]bool {
	allowed := map[string]bool{}
	for _, skill := range skills {
		addAllowedTool(allowed, skill.Name)
		if len(skill.Schema) == 0 || !json.Valid(skill.Schema) {
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(skill.Schema, &schema); err != nil {
			continue
		}
		for _, key := range []string{"tool", "tools", "default_tools", "allowed_tools"} {
			addSchemaTools(allowed, schema[key])
		}
		if category, ok := schema["category"].(string); ok {
			addToolsForCategory(allowed, category)
		}
	}
	return allowed
}

func addSchemaTools(allowed map[string]bool, value any) {
	switch v := value.(type) {
	case string:
		addAllowedTool(allowed, v)
	case []any:
		for _, item := range v {
			if name, ok := item.(string); ok {
				addAllowedTool(allowed, name)
			}
		}
	}
}

func addAllowedTool(allowed map[string]bool, name string) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if normalized == "" {
		return
	}
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "read_file", "write_file", "run_tests", "analyze_logs", "generate_docs", "create_migration", "search_code", "apply_patch":
		allowed[normalized] = true
	}
}

func addToolsForCategory(allowed map[string]bool, category string) {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "test", "testing", "qa":
		allowed["run_tests"] = true
		allowed["analyze_logs"] = true
	case "database", "db", "migration":
		allowed["create_migration"] = true
		allowed["read_file"] = true
		allowed["write_file"] = true
	case "docs", "documentation":
		allowed["generate_docs"] = true
		allowed["read_file"] = true
	case "code", "file", "git":
		allowed["read_file"] = true
		allowed["write_file"] = true
		allowed["search_code"] = true
		allowed["apply_patch"] = true
	}
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
	for i, snippet := range snippets {
		b.WriteString(fmt.Sprintf("### Snippet %d: %s:%d-%d (score %.2f, %s)\n", i+1, snippet.Path, snippet.StartLine, snippet.EndLine, snippet.Relevance, snippet.Retriever))
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

	parts = append(parts, `# Execution Rules
- Prefer apply_patch for source edits instead of rewriting full files.
- Run tests through run_tests when a change is executable.
- Return structured JSON when the workflow step requests JSON output.`)
	return strings.TrimSpace(strings.Join(parts, "\n\n")), projectRules, nil
}

func (a *PromptAssembler) loadRules(ctx context.Context, projectID string) ([]models.Rule, []models.Rule, error) {
	globalRules := []models.Rule{}
	projectRules := []models.Rule{}

	if a != nil && a.rules != nil {
		rules, err := a.rules.ListByProjectID(ctx, projectID)
		if err != nil {
			return nil, nil, err
		}
		for _, rule := range rules {
			switch rule.Scope {
			case models.RuleScopeGlobal:
				globalRules = append(globalRules, rule)
			default:
				projectRules = append(projectRules, rule)
			}
		}
	}

	// Load project rules from disk (Planned 5.5)
	if a != nil && a.dataRoot != "" && projectID != "" {
		diskRulesDir := filepath.Join(a.dataRoot, "projects", projectID, "rules")
		if entries, err := os.ReadDir(diskRulesDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				contentBytes, err := os.ReadFile(filepath.Join(diskRulesDir, entry.Name()))
				if err != nil || len(contentBytes) == 0 {
					continue
				}
				projectRules = append(projectRules, models.Rule{
					ID:          fmt.Sprintf("disk-%s", entry.Name()),
					Scope:       models.RuleScopeProject,
					Content:     string(contentBytes),
					Enforcement: models.RuleEnforcementStrict,
				})
			}
		}
	}

	return globalRules, projectRules, nil
}

func DetectRuleConflicts(globalRules, localRules []models.Rule) error {
	if len(globalRules) == 0 || len(localRules) == 0 {
		return nil
	}
	conflictPattern := regexp.MustCompile(`(?i)\b(ignore|override|disable|bypass)\b.*\b(global|strict|security|rule)`)
	for _, rule := range localRules {
		if conflictPattern.MatchString(rule.Content) {
			scope := rule.Scope
			if scope == "" {
				scope = "project"
			}
			return fmt.Errorf("%s rule %s conflicts with global governance rules", scope, rule.ID)
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

func (a *PromptAssembler) loadProjectDiskSkills(projectID string) ([]models.Skill, error) {
	if a.dataRoot == "" || projectID == "" {
		return nil, nil
	}
	projSkillsDir := filepath.Join(a.dataRoot, "projects", projectID, "skills")
	regPath := filepath.Join(projSkillsDir, "registry.json")
	var diskSkills []models.Skill

	// Try reading registry.json first
	if raw, err := os.ReadFile(regPath); err == nil {
		var reg struct {
			Skills map[string][]struct {
				ID          string          `json:"id"`
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Path        string          `json:"path"`
				Schema      json.RawMessage `json:"schema,omitempty"`
			} `json:"skills"`
		}
		if err := json.Unmarshal(raw, &reg); err == nil {
			for _, skills := range reg.Skills {
				for _, sk := range skills {
					var schemaMap map[string]any
					if len(sk.Schema) > 0 {
						_ = json.Unmarshal(sk.Schema, &schemaMap)
					}
					if schemaMap == nil {
						schemaMap = make(map[string]any)
					}
					schemaMap["source"] = "project_disk"
					schemaMap["category"] = "project"
					schemaMap["registry"] = sk.ID
					schemaMap["path"] = filepath.ToSlash(filepath.Join("projects", projectID, "skills", sk.Path))
					
					schemaRaw, _ := json.Marshal(schemaMap)
					diskSkills = append(diskSkills, models.Skill{
						ID:          sk.ID,
						Name:        sk.Name,
						Description: sk.Description,
						Schema:      schemaRaw,
					})
				}
			}
			return diskSkills, nil
		}
	}

	// Fallback/Direct scan of .md files in the skills directory
	entries, err := os.ReadDir(projSkillsDir)
	if err != nil {
		return nil, nil
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		skillName := strings.TrimSuffix(entry.Name(), ".md")
		schemaRaw, _ := json.Marshal(map[string]string{
			"source": "project_disk",
			"path":   filepath.ToSlash(filepath.Join("projects", projectID, "skills", entry.Name())),
		})
		diskSkills = append(diskSkills, models.Skill{
			ID:          "project-" + skillName,
			Name:        skillName,
			Description: fmt.Sprintf("Project specific skill for %s", skillName),
			Schema:      schemaRaw,
		})
	}

	return diskSkills, nil
}

func (a *PromptAssembler) loadProjectKnowledgeBaseDocs(projectID string, query string) string {
	if a.dataRoot == "" || projectID == "" {
		return ""
	}
	docsDir := filepath.Join(a.dataRoot, "projects", projectID, "docs")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return ""
	}

	queryLower := strings.ToLower(query)
	var sb strings.Builder

	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".md") && !strings.HasSuffix(entry.Name(), ".txt")) {
			continue
		}
		docName := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".md"), ".txt")
		docNameLower := strings.ToLower(docName)

		contentBytes, err := os.ReadFile(filepath.Join(docsDir, entry.Name()))
		if err != nil || len(contentBytes) == 0 {
			continue
		}

		// Simple keyword-based relevance check
		isRelevant := strings.Contains(queryLower, docNameLower)
		if !isRelevant {
			words := strings.FieldsFunc(queryLower, func(r rune) bool {
				return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == '?' || r == '!' || r == '/' || r == '\\' || r == '-' || r == '_'
			})
			for _, word := range words {
				if len(word) > 4 && strings.Contains(docNameLower, word) {
					isRelevant = true
					break
				}
			}
		}

		if isRelevant {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(fmt.Sprintf("--- Knowledge Base: %s ---\n%s", entry.Name(), string(contentBytes)))
		}
	}

	return sb.String()
}

