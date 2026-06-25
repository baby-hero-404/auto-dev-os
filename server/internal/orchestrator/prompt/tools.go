package prompt

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (a *PromptAssembler) toolDefinitionsForAgent(ctx context.Context, agent *models.Agent, projectID string, requiredSkills []string) ([]llm.ToolDefinition, error) {
	if agent == nil || a == nil {
		if a != nil && a.baseTools != nil {
			return a.baseTools, nil
		}
		return []llm.ToolDefinition{}, nil
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
	
	base := a.baseTools
	if base == nil {
		base = []llm.ToolDefinition{}
	}
	return FilterToolsBySkills(base, skills), nil
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

func FilterToolsBySkills(tools []llm.ToolDefinition, skills []models.Skill) []llm.ToolDefinition {
	allowed := allowedToolSetFromSkills(skills)
	if len(allowed) == 0 {
		allowed = map[string]bool{"read_file": true, "write_file": true}
	}
	filtered := make([]llm.ToolDefinition, 0, len(tools))
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
