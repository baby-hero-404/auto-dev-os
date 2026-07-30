package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type LearnedSkillsLister interface {
	SearchActiveByText(ctx context.Context, projectID string, query string, limit int) ([]models.LearnedSkill, error)
}

func RenderLearnedSkillsSection(skills []models.LearnedSkill) string {
	if len(skills) == 0 {
		return ""
	}
	const learnedSkillsCharBudget = 8000
	var sb strings.Builder
	sb.WriteString("## Learned skills (from past tasks in this project)\n")
	for _, sk := range skills {
		section := fmt.Sprintf("### %s\n%s\n\n", sk.Title, sk.Content)
		if sb.Len()+len(section) > learnedSkillsCharBudget {
			break
		}
		sb.WriteString(section)
	}
	return sb.String()
}

func (a *PromptAssembler) learnedSkillsText(ctx context.Context, task models.Task) string {
	if a.learnedSkills == nil {
		return ""
	}
	query := task.Title + "\n" + task.Description
	skills, err := a.learnedSkills.SearchActiveByText(ctx, task.ProjectID, query, 3)
	if err != nil || len(skills) == 0 {
		return ""
	}
	return RenderLearnedSkillsSection(skills)
}

const cliContextCharBudget = 12000

func (a *PromptAssembler) MaterializeCLIContext(ctx context.Context, task models.Task, agent *models.Agent, stepID string) (map[string]string, error) {
	files := make(map[string]string)

	skills, err := a.resolveSkills(ctx, task, agent, stepID)
	if err != nil {
		skills = nil // graceful degradation, not a hard failure (REQ-002)
	}

	var analysis models.TaskAnalysis
	_ = json.Unmarshal(task.Analysis, &analysis) // best-effort; zero value is fine if absent/invalid

	budget := cliContextCharBudget
	includedSkills := make([]string, 0, len(skills))
	slugCounts := make(map[string]int)
	for _, sk := range skills {
		if sk.Content == "" {
			continue
		}
		slug := slugifySkillName(sk.Name)
		slugCounts[slug]++
		if n := slugCounts[slug]; n > 1 {
			slug = fmt.Sprintf("%s-%d", slug, n) // avoid silently clobbering same-slug skills (e.g. "API Patterns" vs "api_patterns")
		}
		path := fmt.Sprintf("relevant/skills/%s.md", slug)
		if budget-len(sk.Content) < 0 {
			break // lowest-scored (resolveSkills returns highest-score first) get dropped first
		}
		files[path] = sk.Content
		budget -= len(sk.Content)
		includedSkills = append(includedSkills, sk.Name)
	}

	if learned := a.learnedSkillsText(ctx, task); learned != "" && budget-len(learned) >= 0 {
		files["relevant/learned_skills.md"] = learned
		budget -= len(learned)
	}

	if len(analysis.TaskRules) > 0 {
		if raw, err := json.MarshalIndent(analysis.TaskRules, "", "  "); err == nil {
			files["relevant/task_rules.md"] = string(raw)
		}
	}

	if len(files) == 0 {
		return map[string]string{}, nil // REQ-002
	}

	manifest := map[string]any{
		"task":      map[string]any{"id": task.ID, "type": stepID},
		"available": map[string]any{"skills": len(includedSkills), "learned_skills": files["relevant/learned_skills.md"] != "", "rules": len(analysis.TaskRules)},
		"sources":   sortedKeys(files),
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	files["manifest.json"] = string(manifestJSON)
	files["README.md"] = "Platform context has been materialized for this task.\n\nInspect `manifest.json` for what's available, then read whatever under `relevant/` is useful for the current step. You decide what to use — nothing here is mandatory.\n"

	return files, nil
}

// slugifySkillName turns an arbitrary, untrusted skill name (sourced from
// third-party git repos' skill markdown frontmatter) into a safe filename
// component. Only [a-z0-9-] survive — anything else (including "/" and "..",
// which would otherwise let a crafted skill name escape .autocode/context/
// when joined into a file path) collapses to a single "-".
func slugifySkillName(name string) string {
	name = strings.ToLower(name)
	var sb strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				sb.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(sb.String(), "-")
	if slug == "" {
		slug = "skill"
	}
	return slug
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
