package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	ID    string   `yaml:"id"`
	Roles []string `yaml:"roles"`
}

func ParseRuleFrontmatter(r *models.Rule) {
	content := strings.TrimSpace(r.Content)
	if !strings.HasPrefix(content, "---") {
		return
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return
	}
	yamlBlock := parts[1]
	bodyBlock := parts[2]

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err == nil {
		if fm.ID != "" {
			r.ID = fm.ID
		}
		if len(fm.Roles) > 0 {
			r.Roles = fm.Roles
		}
		r.Content = strings.TrimSpace(bodyBlock)
	}
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
			ParseRuleFrontmatter(&rule)
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
				r := models.Rule{
					ID:          fmt.Sprintf("disk-%s", entry.Name()),
					Scope:       models.RuleScopeProject,
					Content:     string(contentBytes),
					Enforcement: models.RuleEnforcementStrict,
				}
				ParseRuleFrontmatter(&r)
				projectRules = append(projectRules, r)
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

func formatRules(rules []models.Rule) string {
	lines := make([]string, 0, len(rules))
	for _, rule := range rules {
		lines = append(lines, fmt.Sprintf("- [%s/%s] %s", rule.Scope, rule.Enforcement, strings.TrimSpace(rule.Content)))
	}
	return strings.Join(lines, "\n")
}

// codingStepExcludedRulePatterns lists phrases that indicate rules the LLM
// cannot follow in a single-shot JSON coding step (no execution capability,
// no user interaction, no skill loading).
var codingStepExcludedRulePatterns = []string{
	"run tests",
	"run local tests",
	"linting",
	"Progressive Discovery",
	"JIT Knowledge",
	"Socratic Gate",
	"ask the user",
	"ask at least 3",
	"Update ARCHITECTURE",
	"Document architectural",
}

// filterRulesForAgent removes rules that are impossible for the LLM to
// follow during coding steps, and filters rules by agent roles if specified.
func filterRulesForAgent(rules []models.Rule, agent *models.Agent, stepID string) []models.Rule {
	var filtered []models.Rule
	for _, r := range rules {
		// 1. Check Role constraint if specified
		if len(r.Roles) > 0 {
			if agent == nil {
				continue
			}
			matched := false
			roleLower := strings.ToLower(agent.Role)
			for _, role := range r.Roles {
				if strings.ToLower(role) == roleLower {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 2. Check Step exclusion constraints (original logic)
		if isCodingStep(stepID) {
			excluded := false
			lower := strings.ToLower(r.Content)
			for _, pattern := range codingStepExcludedRulePatterns {
				if strings.Contains(lower, strings.ToLower(pattern)) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		filtered = append(filtered, r)
	}
	return filtered
}

func (a *PromptAssembler) loadProjectDiskSkills(projectID string) ([]models.Skill, error) {
	if a == nil || a.dataRoot == "" || projectID == "" {
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
	if a == nil || a.dataRoot == "" || projectID == "" {
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
