package steps

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func parseAnalysisFinal(parsedFinal map[string]any) models.TaskAnalysis {
	var analysis models.TaskAnalysis
	if comp, ok := parsedFinal["complexity"].(string); ok {
		analysis.Complexity = comp
	}
	if cat, ok := parsedFinal["primary_category"].(string); ok {
		analysis.PrimaryCategory = cat
	}
	if scope, ok := parsedFinal["scope"].(string); ok {
		analysis.Scope = scope
	}
	if aff, ok := parsedFinal["affected_files"].([]any); ok {
		for _, item := range aff {
			if s, ok := item.(string); ok {
				analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{File: s})
			} else if m, ok := item.(map[string]any); ok {
				repo, _ := m["repo"].(string)
				file, _ := m["file"].(string)
				conf, _ := m["confidence"].(float64)
				reason, _ := m["reason"].(string)
				analysis.AffectedFiles = append(analysis.AffectedFiles, models.AffectedFile{
					Repo:       repo,
					File:       file,
					Confidence: conf,
					Reason:     reason,
				})
			}
		}
	}
	if risks, ok := parsedFinal["risks"].([]any); ok {
		for _, item := range risks {
			if s, ok := item.(string); ok {
				analysis.Risks = append(analysis.Risks, s)
			}
		}
	}
	if phases, ok := parsedFinal["execution_phases"].([]any); ok {
		for i, phaseItem := range phases {
			if pMap, ok := phaseItem.(map[string]any); ok {
				phaseName, _ := pMap["phase"].(string)
				var tasks []string
				if tArr, ok := pMap["tasks"].([]any); ok {
					for j, t := range tArr {
						if ts, ok := t.(string); ok {
							tasks = append(tasks, normalizeTaskID(ts, i+1, j+1))
						}
					}
				}
				analysis.ExecutionPhases = append(analysis.ExecutionPhases, models.ExecutionPhase{
					Phase: phaseName,
					Tasks: tasks,
				})
			}
		}
	}
	if units, ok := parsedFinal["execution_units"].([]any); ok {
		for _, unitItem := range units {
			if uMap, ok := unitItem.(map[string]any); ok {
				var unit models.ExecutionUnit
				unit.ID, _ = uMap["id"].(string)
				unit.Objective, _ = uMap["objective"].(string)
				if tArr, ok := uMap["tasks"].([]any); ok {
					for _, t := range tArr {
						if ts, ok := t.(string); ok {
							unit.Tasks = append(unit.Tasks, normalizeTaskID(ts, 0, 0))
						}
					}
				}
				if prof, ok := uMap["execution_profile"].(map[string]any); ok {
					unit.ExecutionProfile.Agent, _ = prof["agent"].(string)
					if sks, ok := prof["skills"].([]any); ok {
						for _, sk := range sks {
							if sksStr, ok := sk.(string); ok {
								unit.ExecutionProfile.Skills = append(unit.ExecutionProfile.Skills, sksStr)
							}
						}
					}
				}
				if cons, ok := uMap["constraints"].(map[string]any); ok {
					unit.Constraints.Parallelizable, _ = cons["parallelizable"].(bool)
					if mf, ok := cons["max_files"].(float64); ok {
						unit.Constraints.MaxFiles = int(mf)
					}
					if et, ok := cons["estimated_tokens"].(float64); ok {
						unit.Constraints.EstimatedTokens = int(et)
					}
					unit.Constraints.MaxRisk, _ = cons["max_risk"].(string)
				}
				if deps, ok := uMap["dependencies"].([]any); ok {
					for _, dep := range deps {
						if depStr, ok := dep.(string); ok {
							unit.Dependencies = append(unit.Dependencies, depStr)
						}
					}
				}
				analysis.ExecutionUnits = append(analysis.ExecutionUnits, unit)
			}
		}
	}
	// Runtime Adapter: Map ExecutionUnits to ExecutionPhases for old UI compatibility
	if len(analysis.ExecutionPhases) == 0 && len(analysis.ExecutionUnits) > 0 {
		for _, unit := range analysis.ExecutionUnits {
			analysis.ExecutionPhases = append(analysis.ExecutionPhases, models.ExecutionPhase{
				Phase: fmt.Sprintf("%s (%s)", unit.Objective, unit.ExecutionProfile.Agent),
				Tasks: unit.Tasks,
			})
		}
	}
	if questions, ok := parsedFinal["clarification_questions"].([]any); ok {
		for _, item := range questions {
			if s, ok := item.(string); ok {
				analysis.ClarificationQuestions = append(analysis.ClarificationQuestions, s)
			}
		}
	}
	if boundaries, ok := parsedFinal["execution_boundaries"]; ok {
		if arr, ok := boundaries.([]any); ok {
			for _, b := range arr {
				if bmap, ok := b.(map[string]any); ok {
					var boundary models.ExecutionBoundary
					if m, ok := bmap["module"].(string); ok {
						boundary.Module = m
					}
					if r, ok := bmap["root"].(string); ok {
						boundary.Root = r
					}
					if rn, ok := bmap["repo_name"].(string); ok {
						boundary.RepoName = rn
					}
					if rid, ok := bmap["repository_id"].(string); ok {
						boundary.RepositoryID = rid
					}
					if caps, ok := bmap["capabilities"].([]any); ok {
						for _, cp := range caps {
							if cps, ok := cp.(string); ok {
								boundary.Capabilities = append(boundary.Capabilities, cps)
							}
						}
					}
					analysis.ExecutionBoundaries = append(analysis.ExecutionBoundaries, boundary)
				}
			}
		} else if bmap, ok := boundaries.(map[string]any); ok {
			// Backward compatibility: map[string][]string
			for k, v := range bmap {
				var boundary models.ExecutionBoundary
				boundary.Module = k
				if arr, ok := v.([]any); ok && len(arr) > 0 {
					if firstPath, ok := arr[0].(string); ok {
						boundary.Root = filepath.Dir(firstPath) + "/"
					}
					for _, p := range arr {
						if _, ok := p.(string); ok {
							boundary.Capabilities = []string{"modify_existing", "create_test"}
						}
					}
				}
				analysis.ExecutionBoundaries = append(analysis.ExecutionBoundaries, boundary)
			}
		}
	}
	if criteria, ok := parsedFinal["acceptance_criteria"].([]any); ok {
		for _, c := range criteria {
			if cmap, ok := c.(map[string]any); ok {
				analysis.AcceptanceCriteria = append(analysis.AcceptanceCriteria, cmap)
			}
		}
	}
	if skills, ok := parsedFinal["required_skills"].([]any); ok {
		for _, item := range skills {
			if s, ok := item.(string); ok {
				analysis.RequiredSkills = append(analysis.RequiredSkills, s)
			}
		}
	}
	if domains, ok := parsedFinal["risk_domains"].([]any); ok {
		for _, item := range domains {
			if s, ok := item.(string); ok {
				analysis.RiskDomains = append(analysis.RiskDomains, s)
			}
		}
	}
	if proposal, ok := parsedFinal["proposal_md"].(string); ok {
		analysis.ProposalMD = proposal
	}
	if specs, ok := parsedFinal["specs_md"].(string); ok {
		analysis.SpecsMD = specs
	}
	if design, ok := parsedFinal["design_md"].(string); ok {
		analysis.DesignMD = design
	}
	if tasks, ok := parsedFinal["tasks_md"].(string); ok {
		analysis.TasksMD = tasks
	}
	return analysis
}

func deriveWorkflowAnalysis(task *models.Task) models.TaskAnalysis {
	text := strings.ToLower(task.Title + " " + task.Description)
	complexity := task.Complexity
	if complexity == "" {
		complexity = models.TaskComplexityEasy
	}
	hardSignals := []string{"architecture", "security", "auth", "permission", "rbac", "payment", "migration", "distributed"}
	mediumSignals := []string{"feature", "refactor", "api", "database", "ui", "workflow", "integration"}
	for _, signal := range hardSignals {
		if strings.Contains(text, signal) {
			complexity = models.TaskComplexityHard
			break
		}
	}
	if complexity != models.TaskComplexityHard {
		for _, signal := range mediumSignals {
			if strings.Contains(text, signal) {
				complexity = models.TaskComplexityMedium
				break
			}
		}
	}
	questions := []string{}
	if len(strings.TrimSpace(task.Description)) < 30 {
		questions = append(questions, "Please provide more implementation context, affected module names, and expected behavior.")
	}
	return models.TaskAnalysis{
		Complexity:    complexity,
		Scope:         "Generated by the Phase 3b workflow analyze step.",
		AffectedFiles: []models.AffectedFile{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPhases: []models.ExecutionPhase{
			{
				Phase: "Automated Execution",
				Tasks: []string{
					"Assemble prompt with role, rules, and retrieved context.",
					"Decompose work into typed subtasks.",
					"Run backend and frontend coding tracks in parallel sandboxes.",
					"Merge, review, fix, test, and prepare PR approval checkpoint.",
				},
			},
		},
		ClarificationQuestions: questions,
	}
}

var taskPrefixRegex = regexp.MustCompile(`^(?i)task[\s-]*(\d+)\.(\d+)[:\-\s]*`)

func normalizeTaskID(taskStr string, fallbackPhase, fallbackTask int) string {
	taskStr = strings.TrimSpace(taskStr)
	if matches := taskPrefixRegex.FindStringSubmatch(taskStr); len(matches) > 0 {
		content := strings.TrimSpace(taskStr[len(matches[0]):])
		return fmt.Sprintf("Task %s.%s: %s", matches[1], matches[2], content)
	}
	if fallbackPhase > 0 && fallbackTask > 0 {
		return fmt.Sprintf("Task %d.%d: %s", fallbackPhase, fallbackTask, taskStr)
	}
	return taskStr
}
