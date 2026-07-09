package prompts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gopkg.in/yaml.v3"
)

// PromptSection represents a modular piece of the final prompt.
type PromptSection struct {
	Name        string
	Body        string
	Priority    int    // Higher integer = lower priority (pruned first)
	RenderOrder int    // Logical order of this section in the rendered prompt
	Tokens      int    // Estimated token count (len(Body)/4)
	IsImmutable bool   // If true, cannot be truncated or dropped
	Destination string // "system" or "user"
}

// ParsedSkill represents a skill file parsed from its markdown/frontmatter.
type ParsedSkill struct {
	ID           string
	Name         string
	Description  string
	AllowedTools []string
	Content      string
	Source       string
}

// EstimateTokens calculates tokens using the 1 token = 4 characters heuristic.
func EstimateTokens(body string) int {
	return len(body) / 4
}

// NewPromptSection creates a PromptSection and estimates its token count.
func NewPromptSection(name string, body string, priority int, renderOrder int, isImmutable bool, dest string) PromptSection {
	return PromptSection{
		Name:        name,
		Body:        body,
		Priority:    priority,
		RenderOrder: renderOrder,
		Tokens:      EstimateTokens(body),
		IsImmutable: isImmutable,
		Destination: dest,
	}
}

// loadStepPromptWithFallback implements fallback-based step prompt loading (REQ-004).
func (a *PromptAssembler) loadStepPromptWithFallback(stepID string) (string, error) {
	if a.promptPaths == nil || a.fs == nil {
		return "", fmt.Errorf("prompt paths or filesystem not configured")
	}

	// Try loading the exact stepID prompt (e.g., plan_v2.md)
	filePath := a.promptPaths.StepPrompt(stepID)
	content, err := a.fs.ReadFile(filePath)
	if err == nil {
		return string(content), nil
	}

	// Seamless fallback: split by '_' and try the base name (e.g., plan.md)
	if strings.Contains(stepID, "_") {
		parts := strings.Split(stepID, "_")
		fallbackID := parts[0]
		filePathFallback := a.promptPaths.StepPrompt(fallbackID)
		contentFallback, errFallback := a.fs.ReadFile(filePathFallback)
		if errFallback == nil {
			return string(contentFallback), nil
		}
	}

	return "", err
}

// parseSkillFile reads and parses the frontmatter and content of a skill markdown file.
func parseSkillFile(filePath string, source string) (ParsedSkill, error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return ParsedSkill{}, err
	}
	content := string(contentBytes)

	ps := ParsedSkill{
		Content: content,
		Source:  source,
	}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			yamlBlock := parts[1]
			bodyBlock := parts[2]

			var fm struct {
				Name         string `yaml:"name"`
				Description  string `yaml:"description"`
				AllowedTools any    `yaml:"allowed-tools"`
			}
			if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err == nil {
				ps.Name = fm.Name
				ps.Description = fm.Description
				ps.Content = strings.TrimSpace(bodyBlock)

				switch v := fm.AllowedTools.(type) {
				case string:
					for _, t := range strings.Split(v, ",") {
						ps.AllowedTools = append(ps.AllowedTools, strings.TrimSpace(t))
					}
				case []any:
					for _, item := range v {
						if name, ok := item.(string); ok {
							ps.AllowedTools = append(ps.AllowedTools, strings.TrimSpace(name))
						}
					}
				}
			}
		}
	}

	if ps.Name == "" {
		ps.Name = strings.TrimSuffix(filepath.Base(filePath), ".md")
	}
	return ps, nil
}

// loadAllSkills merges global skill registry with project-local skills (REQ-006).
func (a *PromptAssembler) loadAllSkills(ctx context.Context, task models.Task) ([]ParsedSkill, error) {
	var mergedSkills []ParsedSkill
	seenNames := make(map[string]bool)

	// In unit tests without rules repo, avoid loading global/local skills to prevent test pollution
	isUnitTest := a.rules == nil
	projectID := task.ProjectID

	// 1. Read Project-Local skills from [ProjectRoot]/skills/ inside task workspace repos
	if !isUnitTest {
		var loadedFromWorkspace bool
		if task.ID != "" && a.dataRoot != "" {
			workspacesDir := filepath.Join(a.dataRoot, "workspaces", task.ID, "code", "repos")
			if repoEntries, err := os.ReadDir(workspacesDir); err == nil {
				for _, repoEntry := range repoEntries {
					if !repoEntry.IsDir() {
						continue
					}
					repoPath := filepath.Join(workspacesDir, repoEntry.Name())
					if branchEntries, err := os.ReadDir(repoPath); err == nil {
						for _, branchEntry := range branchEntries {
							if !branchEntry.IsDir() || branchEntry.Name() == "worktrees" {
								continue
							}
							localSkillsDir := filepath.Join(repoPath, branchEntry.Name(), "skills")
							if entries, err := os.ReadDir(localSkillsDir); err == nil {
								for _, entry := range entries {
									if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
										continue
									}
									filePath := filepath.Join(localSkillsDir, entry.Name())
									if ps, err := parseSkillFile(filePath, "project_local"); err == nil {
										if !seenNames[strings.ToLower(ps.Name)] {
											mergedSkills = append(mergedSkills, ps)
											seenNames[strings.ToLower(ps.Name)] = true
											loadedFromWorkspace = true
										}
									}
								}
							}
						}
					}
				}
			}
		}
		// Fallback to relative skills folder if nothing loaded from workspace
		if !loadedFromWorkspace {
			localSkillsDir := filepath.Join(".", "skills")
			if entries, err := os.ReadDir(localSkillsDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
						continue
					}
					filePath := filepath.Join(localSkillsDir, entry.Name())
					if ps, err := parseSkillFile(filePath, "project_local"); err == nil {
						if !seenNames[strings.ToLower(ps.Name)] {
							mergedSkills = append(mergedSkills, ps)
							seenNames[strings.ToLower(ps.Name)] = true
						}
					}
				}
			}
		}
	}

	// 2. Read Central Global Skill Registry (usually under ~/.gemini/)
	if !isUnitTest {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			registryPath := filepath.Join(homeDir, ".gemini", "registry.min.json")
			if raw, err := os.ReadFile(registryPath); err == nil {
				var reg struct {
					Skills struct {
						Core []struct {
							ID   string `json:"id"`
							Path string `json:"path"`
						} `json:"core"`
						Tech []struct {
							ID   string `json:"id"`
							Path string `json:"path"`
						} `json:"tech"`
					} `json:"skills"`
				}
				if err := json.Unmarshal(raw, &reg); err == nil {
					var globalItems []struct {
						ID   string
						Path string
					}
					for _, s := range reg.Skills.Core {
						globalItems = append(globalItems, struct {
							ID   string
							Path string
						}{ID: s.ID, Path: s.Path})
					}
					for _, s := range reg.Skills.Tech {
						globalItems = append(globalItems, struct {
							ID   string
							Path string
						}{ID: s.ID, Path: s.Path})
					}

					for _, gi := range globalItems {
						if seenNames[strings.ToLower(gi.ID)] {
							continue
						}
						skillFile := filepath.Join(homeDir, ".gemini", gi.Path, "SKILL.md")
						if ps, err := parseSkillFile(skillFile, "global"); err == nil {
							ps.ID = gi.ID
							mergedSkills = append(mergedSkills, ps)
							seenNames[strings.ToLower(ps.Name)] = true
						}
					}
				}
			}
		}
	}

	// 3. Read Database Project-Disk skills from disk/GORM
	diskSkills, err := a.loadProjectDiskSkills(projectID)
	if err == nil {
		for _, ds := range diskSkills {
			if seenNames[strings.ToLower(ds.Name)] {
				continue
			}

			// Read schema to get path
			var schema map[string]any
			_ = json.Unmarshal(ds.Schema, &schema)
			if schemaPath, ok := schema["path"].(string); ok {
				skillPath := filepath.Join(a.dataRoot, schemaPath)
				if ps, err := parseSkillFile(skillPath, "project_disk"); err == nil {
					mergedSkills = append(mergedSkills, ps)
					seenNames[strings.ToLower(ps.Name)] = true
				}
			}
		}
	}

	// 4. Query a.skills database-backed list to support unit tests and database lister
	if a.skills != nil {
		if dbSkills, err := a.skills.List(ctx); err == nil {
			for _, s := range dbSkills {
				if seenNames[strings.ToLower(s.Name)] {
					continue
				}
				var allowedTools []string
				if len(s.Schema) > 0 && json.Valid(s.Schema) {
					var schema map[string]any
					if err := json.Unmarshal(s.Schema, &schema); err == nil {
						for _, key := range []string{"tool", "tools", "default_tools", "allowed_tools"} {
							if val, ok := schema[key]; ok {
								switch v := val.(type) {
								case string:
									for _, t := range strings.Split(v, ",") {
										allowedTools = append(allowedTools, strings.TrimSpace(t))
									}
								case []any:
									for _, item := range v {
										if name, ok := item.(string); ok {
											allowedTools = append(allowedTools, strings.TrimSpace(name))
										}
									}
								}
							}
						}
					}
				}

				ps := ParsedSkill{
					ID:           s.Name,
					Name:         s.Name,
					Description:  s.Description,
					AllowedTools: allowedTools,
					Content:      s.Description, // Fallback content
					Source:       "database",
				}
				mergedSkills = append(mergedSkills, ps)
				seenNames[strings.ToLower(ps.Name)] = true
			}
		}
	}

	return mergedSkills, nil
}

// resolveSkills determines the 3-5 most relevant JIT skills (REQ-003).
func (a *PromptAssembler) resolveSkills(ctx context.Context, task models.Task, agent *models.Agent, stepID string) ([]ParsedSkill, error) {
	allSkills, err := a.loadAllSkills(ctx, task)
	if err != nil {
		return nil, err
	}

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	roleLower := ""
	if agent != nil {
		roleLower = strings.ToLower(agent.Role)
	}

	type ScoredSkill struct {
		Skill ParsedSkill
		Score int
	}
	var scored []ScoredSkill

	for _, sk := range allSkills {
		score := 0
		skNameLower := strings.ToLower(sk.Name)

		// 1. Role match
		if roleLower != "" && isSkillMatchingRole(sk.Name, agent.Role) {
			score += 10
		}

		// 2. Required skills map match
		if roleLower != "" && len(analysis.RequiredSkillsMap) > 0 {
			for _, req := range analysis.RequiredSkillsMap[roleLower] {
				if strings.ToLower(req) == skNameLower {
					score += 15
				}
			}
		}

		// 3. Keyword matches in Title/Description
		titleLower := strings.ToLower(task.Title)
		descLower := strings.ToLower(task.Description)
		if strings.Contains(titleLower, skNameLower) {
			score += 5
		}
		if strings.Contains(descLower, skNameLower) {
			score += 3
		}

		// 4. Keyword matches in Step ID
		if stepID != "" && strings.Contains(strings.ToLower(stepID), skNameLower) {
			score += 2
		}

		if score > 0 {
			scored = append(scored, ScoredSkill{Skill: sk, Score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Skill.Name < scored[j].Skill.Name
		}
		return scored[i].Score > scored[j].Score
	})

	limit := 5
	if len(scored) < limit {
		limit = len(scored)
	}

	resolved := make([]ParsedSkill, 0, limit)
	for i := 0; i < limit; i++ {
		resolved = append(resolved, scored[i].Skill)
	}

	return resolved, nil
}

// collect gathers all prompt sections for the given task and agent.
func (a *PromptAssembler) collect(ctx context.Context, task models.Task, agent *models.Agent) ([]PromptSection, error) {
	stepID := stepIDFromCtx(ctx)
	var sections []PromptSection

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	// Overwrite mutable analysis fields with FrozenContext if present (REQ-M01)
	if inputs := StepInputsFromCtx(ctx); inputs != nil {
		if planOut, ok := inputs[workflow.StepPlan]; ok {
			if frozenJSON, ok := planOut["frozen_context"].(string); ok && frozenJSON != "" {
				var frozen models.FrozenContext
				if err := json.Unmarshal([]byte(frozenJSON), &frozen); err == nil {
					analysis.SpecHash = frozen.SpecHash
					analysis.ProposalMD = frozen.ProposalMD
					analysis.SpecsMD = frozen.SpecsMD
					analysis.DesignMD = frozen.DesignMD
					analysis.TasksMD = frozen.TasksMD
					analysis.ExecutionUnits = frozen.ExecutionUnits
					analysis.ExecutionBoundaries = frozen.ExecutionBoundaries
					analysis.AffectedFiles = frozen.AffectedFiles
					analysis.AcceptanceCriteria = frozen.AcceptanceCriteria
					analysis.ExecutionPhases = frozen.ExecutionPhases
					analysis.Risks = frozen.Risks
					analysis.RiskDomains = frozen.RiskDomains
				}
			}
		}
	}

	// 1. Base Prompt (Immutable)
	if a.promptPaths != nil && a.fs != nil {
		corePromptFile := a.promptPaths.CorePrompt("system_prompt.md")
		if content, err := a.fs.ReadFile(corePromptFile); err == nil {
			sections = append(sections, NewPromptSection("Base Prompt", string(content), 10, 10, true, "system"))
		}
	}

	// 2. Role Prompt (Immutable)
	if agent != nil && a.promptPaths != nil && a.fs != nil {
		rolePromptFile := a.promptPaths.RolePrompt(agent.Role)
		if content, err := a.fs.ReadFile(rolePromptFile); err == nil {
			sections = append(sections, NewPromptSection("Role Prompt", string(content), 20, 20, true, "system"))
		}
	}

	// 3. Step Prompt
	if stepID != "" {
		if content, err := a.loadStepPromptWithFallback(stepID); err == nil && content != "" {
			sections = append(sections, NewPromptSection("Step Prompt", content, 50, 30, false, "system"))
		}
	}

	// 4. JIT Skills
	resolvedJITSkills, err := a.resolveSkills(ctx, task, agent, stepID)
	if err == nil && len(resolvedJITSkills) > 0 {
		var skBuilder strings.Builder
		skBuilder.WriteString("# JIT Skills\n")
		for _, sk := range resolvedJITSkills {
			skBuilder.WriteString(fmt.Sprintf("## Skill: %s\n%s\n\n", sk.Name, sk.Content))
		}
		sections = append(sections, NewPromptSection("JIT Skills", skBuilder.String(), 60, 40, false, "system"))
	}

	// 5. Layered Rules (REQ-005)
	globalRules, projectRules, err := a.loadRules(ctx, task.ProjectID)
	if err == nil {
		// Conflict Detection
		localRules := append([]models.Rule{}, projectRules...)
		for i, tr := range analysis.TaskRules {
			localRules = append(localRules, models.Rule{
				ID:          fmt.Sprintf("task-rule-%d", i),
				Scope:       "task",
				Content:     tr,
				Enforcement: models.RuleEnforcementStrict,
			})
		}
		_ = DetectRuleConflicts(globalRules, localRules)

		// 5.1 Global Rules (Immutable)
		var globalParts []models.Rule
		for _, r := range globalRules {
			if len(r.Roles) == 0 {
				globalParts = append(globalParts, r)
			}
		}
		if len(globalParts) > 0 {
			filtered := filterRulesForAgent(globalParts, agent, stepID)
			if len(filtered) > 0 {
				sections = append(sections, NewPromptSection("Global Rules", "# Global Rules [IMMUTABLE - DO NOT OVERRIDE]\n"+formatRules(filtered), 15, 50, true, "system"))
			}
		}

		// 5.2 Agent Role Constraints (Immutable)
		if agent != nil {
			var constraintParts []models.Rule
			roleLower := strings.ToLower(agent.Role)
			for _, r := range globalRules {
				if len(r.Roles) > 0 {
					matched := false
					for _, role := range r.Roles {
						if strings.ToLower(role) == roleLower {
							matched = true
							break
						}
					}
					if matched {
						constraintParts = append(constraintParts, r)
					}
				}
			}
			if len(constraintParts) > 0 {
				sections = append(sections, NewPromptSection("Role Constraints", "# Agent Role Constraints\n"+formatRules(constraintParts), 25, 60, true, "system"))
			}
		}

		// 5.3 Project Rules (Strict = Immutable, Advisory = Mutable)
		if len(projectRules) > 0 {
			filtered := filterRulesForAgent(projectRules, agent, stepID)
			if len(filtered) > 0 {
				var strictRules []models.Rule
				var advisoryRules []models.Rule
				for _, r := range filtered {
					if r.Enforcement == models.RuleEnforcementStrict {
						strictRules = append(strictRules, r)
					} else {
						advisoryRules = append(advisoryRules, r)
					}
				}

				if len(strictRules) > 0 {
					sections = append(sections, NewPromptSection("Project Rules (Strict)", "# Project Rules (Strict)\n"+formatRules(strictRules), 35, 70, true, "system"))
				}
				if len(advisoryRules) > 0 {
					sections = append(sections, NewPromptSection("Project Rules (Advisory)", "# Project Rules (Advisory)\n"+formatRules(advisoryRules), 45, 75, false, "system"))
				}
			}
		}

		// 5.4 Task Rules
		if len(analysis.TaskRules) > 0 {
			var b strings.Builder
			b.WriteString("# Task-specific Rules:\n")
			for _, tr := range analysis.TaskRules {
				b.WriteString(fmt.Sprintf("- [task/strict] %s\n", strings.TrimSpace(tr)))
			}
			sections = append(sections, NewPromptSection("Task Rules", b.String(), 55, 80, false, "system"))
		}
	}

	// 6. Output Rules
	if a.promptPaths != nil && a.fs != nil {
		outputRulesFile := a.promptPaths.CorePrompt("output_rules.md")
		if content, err := a.fs.ReadFile(outputRulesFile); err == nil {
			sections = append(sections, NewPromptSection("Output Rules", string(content), 35, 90, true, "system"))
		}
	}

	// 7. Context Builder slices & Sliced Context (REQ-M01, REQ-M02)
	isReviewer := agent != nil && strings.ToLower(agent.Role) == "reviewer"

	// Requirement slice
	reqText := "Task: " + task.Title + "\n\n"
	useOriginalDescription := true
	if (shouldInjectFullSpec(stepID) || isCodingStep(stepID) || stepID == workflow.StepReview || stepID == workflow.StepTest) && (analysis.SpecsMD != "" || analysis.ProposalMD != "") {
		useOriginalDescription = false
	}
	if useOriginalDescription {
		reqText += task.Description
	} else {
		reqText += "> [!IMPORTANT]\n> Original Task Description is omitted. Your evaluation MUST be based strictly on the execution contract and specific context provided in this prompt.\n"
	}
	sections = append(sections, NewPromptSection("Task Requirement", reqText, 30, 10, true, "user"))

	// Answers to Clarifications
	if len(task.Clarifications) > 0 {
		var rounds []models.ClarificationRound
		if err := json.Unmarshal(task.Clarifications, &rounds); err == nil && len(rounds) > 0 {
			var clarBuilder strings.Builder
			clarBuilder.WriteString("=== Answers to Clarification Questions ===\n")
			for _, r := range rounds {
				clarBuilder.WriteString(fmt.Sprintf("#### Round %d:\n%s\n\n", r.Round, r.Response))
			}
			sections = append(sections, NewPromptSection("Clarifications", clarBuilder.String(), 70, 20, false, "user"))
		}
	}

	// Context slices routing matrix depending on capability/role
	if isReviewer {
		// Strict Reviewer context (REQ-M02)
		// Requirement (already added), AC, Coding Standards, Security Checklist, Performance Checklist, Diff
		if analysis.SpecsMD != "" {
			sections = append(sections, NewPromptSection("Acceptance Criteria", "=== Acceptance Criteria ===\n"+analysis.SpecsMD, 40, 30, false, "user"))
		}
		// Diff: construct from memories or task description if no git provider
		if task.Description != "" && strings.Contains(strings.ToLower(task.Description), "diff") {
			sections = append(sections, NewPromptSection("Git Diff", "=== Diff ===\n"+task.Description, 40, 35, false, "user"))
		}
	} else {
		// General Agent Context slices
		if shouldInjectFullSpec(stepID) || isCodingStep(stepID) {
			if analysis.ProposalMD != "" || analysis.SpecsMD != "" || len(analysis.ExecutionPhases) > 0 {
				var specBuilder strings.Builder
				specBuilder.WriteString("=== Task Specification (OpenSpec) ===\n")
				if analysis.ProposalMD != "" {
					specBuilder.WriteString(analysis.ProposalMD + "\n\n")
				}
				if analysis.SpecsMD != "" {
					specBuilder.WriteString(analysis.SpecsMD + "\n\n")
				}
				if analysis.DesignMD != "" {
					specBuilder.WriteString(analysis.DesignMD + "\n\n")
				}
				if len(analysis.Tasks) > 0 {
					specBuilder.WriteString(formatTasksMD(analysis.Tasks) + "\n\n")
				} else if analysis.TasksMD != "" {
					specBuilder.WriteString(analysis.TasksMD + "\n\n")
				}
				if specBuilder.Len() > len("=== Task Specification (OpenSpec) ===\n") {
					sections = append(sections, NewPromptSection("Task Specifications", specBuilder.String(), 40, 30, false, "user"))
				}

				// Inject Execution Manifest (JSON)
				var manifestJSON []byte
				if isCodingStep(stepID) {
					manifest := map[string]any{
						"affected_files": analysis.AffectedFiles,
					}
					if len(analysis.Tasks) > 0 {
						manifest["tasks"] = analysis.Tasks
					}
					manifestJSON, _ = json.MarshalIndent(manifest, "", "  ")
				} else {
					manifest := map[string]any{
						"affected_files": analysis.AffectedFiles,
						"risks":          analysis.Risks,
					}
					if len(analysis.ExecutionPhases) > 0 {
						manifest["execution_phases"] = analysis.ExecutionPhases
					}
					if len(analysis.Tasks) > 0 {
						manifest["tasks"] = analysis.Tasks
					}
					if len(analysis.RiskDomains) > 0 {
						manifest["risk_domains"] = analysis.RiskDomains
					}
					if len(analysis.AcceptanceCriteria) > 0 {
						manifest["acceptance_criteria"] = analysis.AcceptanceCriteria
					}
					if len(analysis.ExecutionBoundaries) > 0 {
						manifest["execution_boundaries"] = analysis.ExecutionBoundaries
					}
					manifestJSON, _ = json.MarshalIndent(manifest, "", "  ")
				}

				if len(manifestJSON) > 0 {
					sections = append(sections, NewPromptSection("Execution Manifest", "## Execution Manifest (JSON):\n```json\n"+string(manifestJSON)+"\n```\n\n", 40, 35, false, "user"))
				}
			}
		}

		if isCodingStep(stepID) {
			idx, ok := extractSubtaskIndex(stepID)
			if ok && idx >= 0 {
				specSection := extractSpecsSectionForSubtask(analysis.SpecsMD, formatTasksMD(analysis.Tasks), idx, stepID)
				if specSection != "" {
					sections = append(sections, NewPromptSection("Relevant Requirements", "=== Relevant Requirements (OpenSpec) ===\n"+specSection, 40, 40, false, "user"))
				}
				progress := summarizeTasksProgress(formatTasksMD(analysis.Tasks), idx, stepID)
				if progress != "" {
					sections = append(sections, NewPromptSection("Tasks Progress", progress, 80, 50, false, "user"))
				}
			}
		}

		// Check for cached ContextCache from ContextLoadStep (REQ-M02)
		var cachedData *models.ContextCache
		if inputs := StepInputsFromCtx(ctx); inputs != nil {
			if contextLoadOut, ok := inputs[workflow.StepContextLoad]; ok {
				if cacheJSON, ok := contextLoadOut["context_cache"].(string); ok && cacheJSON != "" {
					var cache models.ContextCache
					if err := json.Unmarshal([]byte(cacheJSON), &cache); err == nil {
						cachedData = &cache
					}
				}
			}
		}

		// Semantic Snippets (ContextBuilder) (REQ-M01)
		var contextBlock string
		var activeFiles []string

		if cachedData != nil && len(cachedData.SemanticSnippets) > 0 {
			maxSnippets := 8
			if isCodingStep(stepID) {
				maxSnippets = 4
			}
			snippets := cachedData.SemanticSnippets
			if len(snippets) > maxSnippets {
				snippets = snippets[:maxSnippets]
			}
			snippets = deduplicateSnippets(snippets)
			contextBlock = formatContextSnippets(snippets)
			for _, s := range snippets {
				activeFiles = append(activeFiles, s.Path)
			}
		} else if a.ctxEngine != nil && shouldAttachCodeContext(agent) {
			maxSnippets := 8
			if isCodingStep(stepID) {
				maxSnippets = 4
			}
			snippets, err := a.ctxEngine.RetrieveContext(ctx, task.Title+"\n"+task.Description, maxSnippets)
			if err == nil {
				snippets = deduplicateSnippets(snippets)
				contextBlock = formatContextSnippets(snippets)
				for _, s := range snippets {
					activeFiles = append(activeFiles, s.Path)
				}
			}
		}

		if a.dataRoot != "" && shouldAttachCodeContext(agent) {
			kbContent := a.loadProjectKnowledgeBaseDocs(task.ProjectID, task.Title+"\n"+task.Description)
			if kbContent != "" {
				if contextBlock != "" {
					contextBlock = kbContent + "\n\n" + contextBlock
				} else {
					contextBlock = kbContent
				}
			}
		}

		if contextBlock != "" {
			sections = append(sections, NewPromptSection("Semantic Context", "Semantic Code Retrieval Context:\n"+contextBlock, 100, 60, false, "user"))
		}

		// Repo Map
		if cachedData != nil && cachedData.RepoMap != "" {
			sections = append(sections, NewPromptSection("Repository Structure", "=== Repository Structure ===\n"+cachedData.RepoMap, 100, 5, false, "user"))
		} else if a.ctxEngine != nil && agent != nil && (agent.Role == models.AgentRoleBackend || agent.Role == models.AgentRoleFrontend || agent.Role == models.AgentRoleReviewer) {
			// Pre-calculate tokens in sections to determine available budget for RepoMap
			usedTokens := 0
			for _, sec := range sections {
				usedTokens += sec.Tokens
			}
			maxMapTokens := 8192 - usedTokens
			if maxMapTokens > 2048 {
				maxMapTokens = 2048
			} else if maxMapTokens < 256 {
				maxMapTokens = 256
			}
			repoMap, err := a.ctxEngine.GetRepoMap(ctx, activeFiles, maxMapTokens)
			if err == nil && repoMap != "" {
				sections = append(sections, NewPromptSection("Repository Structure", "=== Repository Structure ===\n"+repoMap, 100, 5, false, "user"))
			}
		}
	}

	// Memories
	if memories, ok := ctx.Value(MemoriesCtxKey).([]models.EpisodicMemory); ok && len(memories) > 0 {
		sections = append(sections, NewPromptSection("Retrieved Memories", "=== Retrieved Memories ===\n"+formatMemories(memories), 90, 70, false, "user"))
	}

	return sections, nil
}

// sort orders sections by RenderOrder for final rendering.
func (a *PromptAssembler) sort(sections []PromptSection) []PromptSection {
	sorted := make([]PromptSection, len(sections))
	copy(sorted, sections)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].RenderOrder < sorted[j].RenderOrder
	})
	return sorted
}

// render compiles the sections into system and user prompts.
func (a *PromptAssembler) render(sections []PromptSection) (string, string) {
	var systemParts []string
	var userParts []string

	for _, sec := range sections {
		if sec.Body == "" {
			continue
		}
		if sec.Destination == "system" {
			systemParts = append(systemParts, sec.Body)
		} else {
			userParts = append(userParts, sec.Body)
		}
	}

	return strings.Join(systemParts, "\n\n"), strings.Join(userParts, "\n\n")
}

// optimizeBudget enforces the maximum token limit by dropping/truncating mutable sections (REQ-002).
func (a *PromptAssembler) optimizeBudget(sections []PromptSection, maxLimit int) []PromptSection {
	totalTokens := 0
	for _, sec := range sections {
		totalTokens += sec.Tokens
	}

	if totalTokens <= maxLimit {
		return sections
	}

	result := make([]PromptSection, len(sections))
	copy(result, sections)

	for totalTokens > maxLimit {
		// Find the mutable section with the highest Priority number (lowest priority)
		worstIdx := -1
		highestPriorityVal := -1

		for i, sec := range result {
			if !sec.IsImmutable && sec.Body != "" {
				if sec.Priority > highestPriorityVal {
					highestPriorityVal = sec.Priority
					worstIdx = i
				}
			}
		}

		if worstIdx == -1 {
			break // All remaining sections are immutable
		}

		// Omit the section
		totalTokens -= result[worstIdx].Tokens
		result[worstIdx].Body = ""
		result[worstIdx].Tokens = 0
	}

	var activeSections []PromptSection
	for _, sec := range result {
		if sec.Body != "" {
			activeSections = append(activeSections, sec)
		}
	}

	return activeSections
}
