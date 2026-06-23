package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/lib/pq"
)

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string) map[string]workflow.StepFunc {
	runners := map[string]workflow.StepFunc{
		workflow.StepContextLoad: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusContextLoading); err != nil {
				return nil, err
			}

			localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
			var repoPaths []string
			ws, errWS := o.LoadTaskWorkspace(ctx, task)
			if errWS == nil {
				for _, rWS := range ws.Repos {
					repoAbs := filepath.Join(ws.Root, rWS.Paths.Main)
					if _, errStat := os.Stat(repoAbs); errStat == nil {
						repoPaths = append(repoPaths, repoAbs)
					}
				}
			}
			if len(repoPaths) == 0 {
				repoPaths = append(repoPaths, localPath)
				if task.RepositoryID == nil {
					if files, err := os.ReadDir(localPath); err == nil {
						for _, f := range files {
							if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
								subPath := filepath.Join(localPath, f.Name())
								if _, errGit := os.Stat(filepath.Join(subPath, ".git")); errGit == nil {
									repoPaths = append(repoPaths, subPath)
								} else if _, errMod := os.Stat(filepath.Join(subPath, "go.mod")); errMod == nil {
									repoPaths = append(repoPaths, subPath)
								} else if _, errPkg := os.Stat(filepath.Join(subPath, "package.json")); errPkg == nil {
									repoPaths = append(repoPaths, subPath)
								}
							}
						}
					}
				}
			}

			result := map[string]any{}
			gitLogs := map[string]string{}
			currentBranches := map[string]string{}
			testCommands := []string{}
			ciConfigs := []string{}
			conventions := map[string]string{}
			architectures := map[string]string{}
			contributings := map[string]string{}

			for _, rp := range repoPaths {
				rel, _ := filepath.Rel(localPath, rp)
				if rel == "." {
					rel = "root"
				}

				var gitLog string
				containerPath := o.containerPathForHostPath(task, rp, "")
				if diffOutput, err := o.runSandboxStep(ctx, task, agent, "get_git_log_"+rel, fmt.Sprintf("git -C %s log -5 --oneline", quoteShellArg(containerPath))); err == nil {
					if stdout, ok := diffOutput["stdout"].(string); ok && stdout != "" {
						gitLog = strings.TrimSpace(stdout)
					}
				}
				if gitLog != "" {
					gitLogs[rel] = gitLog
				}

				var currentBranch string
				if diffOutput, err := o.runSandboxStep(ctx, task, agent, "get_git_branch_"+rel, fmt.Sprintf("git -C %s rev-parse --abbrev-ref HEAD", quoteShellArg(containerPath))); err == nil {
					if stdout, ok := diffOutput["stdout"].(string); ok && stdout != "" {
						currentBranch = strings.TrimSpace(stdout)
					}
				}
				if currentBranch != "" {
					currentBranches[rel] = currentBranch
				}

				if _, err := os.Stat(filepath.Join(rp, "Makefile")); err == nil {
					testCommands = append(testCommands, fmt.Sprintf("make test (in %s)", rel))
				}
				if pJsonData, err := os.ReadFile(filepath.Join(rp, "package.json")); err == nil {
					var pMap map[string]any
					if err := json.Unmarshal(pJsonData, &pMap); err == nil {
						if scripts, ok := pMap["scripts"].(map[string]any); ok {
							if _, ok := scripts["test"]; ok {
								testCommands = append(testCommands, fmt.Sprintf("npm test (in %s)", rel))
							}
							if _, ok := scripts["lint"]; ok {
								result["has_lint_script"] = true
							}
							if _, ok := scripts["build"]; ok {
								result["has_build_script"] = true
							}
						}
					}
				}
				if _, err := os.Stat(filepath.Join(rp, "go.mod")); err == nil {
					testCommands = append(testCommands, fmt.Sprintf("go test ./... (in %s)", rel))
				}

				if files, err := os.ReadDir(filepath.Join(rp, ".github", "workflows")); err == nil {
					for _, f := range files {
						if !f.IsDir() {
							ciConfigs = append(ciConfigs, filepath.Join(rel, ".github", "workflows", f.Name()))
						}
					}
				}
				if _, err := os.Stat(filepath.Join(rp, ".gitlab-ci.yml")); err == nil {
					ciConfigs = append(ciConfigs, filepath.Join(rel, ".gitlab-ci.yml"))
				}

				conventionFiles := []string{".editorconfig", ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".golangci.yml"}
				for _, file := range conventionFiles {
					if data, err := readLimitedFile(filepath.Join(rp, file), 10000); err == nil {
						conventions[filepath.Join(rel, file)] = data
					}
				}

				if data, err := readLimitedFile(filepath.Join(rp, "ARCHITECTURE.md"), 10000); err == nil {
					architectures[rel] = data
				}
				if data, err := readLimitedFile(filepath.Join(rp, "CONTRIBUTING.md"), 10000); err == nil {
					contributings[rel] = data
				}
			}

			result["git_logs"] = gitLogs
			result["current_branches"] = currentBranches
			result["test_commands"] = testCommands
			result["ci_configs"] = ciConfigs
			result["conventions"] = conventions
			result["architectures"] = architectures
			result["contributings"] = contributings

			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepContextLoad, "context", result)

			return result, nil
		},
		workflow.StepAnalyze: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			if o.prompts != nil {
				messages, tools, err := o.prompts.AssembleForAgent(ctx, *task, agent, nil)
				if err != nil {
					return nil, err
				}
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("assembled prompt with %d messages and %d tools", len(messages), len(tools)))
			}
			if taskReadyForExecution(task) {
				return map[string]any{"complexity": task.Complexity, "spec_status": task.SpecStatus}, nil
			}

			var analysis models.TaskAnalysis
			if o.llm != nil {
				instruction := `Analyze this task and output the proposed specification as a valid JSON object.
You must output ONLY a valid JSON object (or inside a ` + "```json" + ` block).
The JSON object MUST have the following structure:
{
  "complexity": "easy" | "medium" | "hard",
  "scope": "A clear, detailed description of the scope of the change",
  "affected_files": ["list", "of", "files", "expected", "to", "be", "modified"],
  "risks": ["list", "of", "potential", "risks", "and", "challenges"],
  "risk_domains": ["list", "of", "risk", "domains", "touched", "(e.g., 'auth', 'payment', 'security', 'data_migration', 'infra', 'rbac', 'public_api')"],
  "execution_plan": ["step-by-step", "plan", "to", "implement", "this", "task"],
  "clarification_questions": ["questions", "if", "more", "details", "are", "needed"],
  "required_skills": ["list", "of", "skill", "names", "required", "for", "this", "task", "(e.g., 'docker_expert', 'frontend_design')"],
  "proposal_md": "Markdown for proposal.md (use the template below)",
  "specs_md": "Markdown for specs.md (use the template below)",
  "design_md": "Markdown for design.md (use the template below)",
  "tasks_md": "Markdown for tasks.md (use the template below)"
}

=== OPENSPEC TEMPLATE: proposal.md ===
## Why
(1-2 sentences: what problem does this solve? Why now?)

## What Changes
(Bullet list of specific changes. Mark breaking changes with **BREAKING**.)

## Capabilities
### New Capabilities
- ` + "`<name>`" + `: <brief description>

### Modified Capabilities
- ` + "`<existing-name>`" + `: <what requirement is changing>

## Impact
(Affected code, APIs, dependencies, systems)

=== OPENSPEC TEMPLATE: specs.md ===
Use delta operations as section headers:
## ADDED Requirements
### Requirement: <name>
<Description using SHALL/MUST language>

#### Scenario: <scenario name>
- **WHEN** <condition>
- **THEN** <expected outcome>

## MODIFIED Requirements
(Same format, include full updated content)

## REMOVED Requirements
### Requirement: <name>
**Reason**: <why removed>
**Migration**: <how to migrate>

=== OPENSPEC TEMPLATE: design.md ===
## Context
(Background, current state, constraints)

## Goals / Non-Goals
**Goals:** ...
**Non-Goals:** ...

## Decisions
(Key technical choices with rationale)

## Risks / Trade-offs
(Known limitations, format: [Risk] → Mitigation)

## Open Questions
(Outstanding decisions or unknowns)

=== OPENSPEC TEMPLATE: tasks.md ===
Group related tasks under numbered headings. Each task MUST be a checkbox.
## 1. <Group Name>
- [ ] 1.1 <Task description>
- [ ] 1.2 <Task description>

## 2. <Group Name>
- [ ] 2.1 <Task description>
`
				var repoContext string
				if contextOut, ok := stepCtx.Inputs[workflow.StepContextLoad]; ok {
					contextJSON, _ := json.Marshal(contextOut)
					repoContext = string(contextJSON)
				}
				if repoContext != "" {
					instruction += "\n\n=== UNTRUSTED REPOSITORY-CONTROLLED CONTEXT (potentially outdated or invalid) ===\n" + repoContext
				}
				res, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepAnalyze, instruction)
				if err == nil {
					if parsed, ok := res["parsed"].(map[string]any); ok {
						if comp, ok := parsed["complexity"].(string); ok {
							analysis.Complexity = comp
						}
						if scope, ok := parsed["scope"].(string); ok {
							analysis.Scope = scope
						}
						if aff, ok := parsed["affected_files"].([]any); ok {
							for _, item := range aff {
								if s, ok := item.(string); ok {
									analysis.AffectedFiles = append(analysis.AffectedFiles, s)
								}
							}
						}
						if risks, ok := parsed["risks"].([]any); ok {
							for _, item := range risks {
								if s, ok := item.(string); ok {
									analysis.Risks = append(analysis.Risks, s)
								}
							}
						}
						if execPlan, ok := parsed["execution_plan"].([]any); ok {
							for _, item := range execPlan {
								if s, ok := item.(string); ok {
									analysis.ExecutionPlan = append(analysis.ExecutionPlan, s)
								}
							}
						}
						if questions, ok := parsed["clarification_questions"].([]any); ok {
							for _, item := range questions {
								if s, ok := item.(string); ok {
									analysis.ClarificationQuestions = append(analysis.ClarificationQuestions, s)
								}
							}
						}
						if skills, ok := parsed["required_skills"].([]any); ok {
							for _, item := range skills {
								if s, ok := item.(string); ok {
									analysis.RequiredSkills = append(analysis.RequiredSkills, s)
								}
							}
						}
						if domains, ok := parsed["risk_domains"].([]any); ok {
							for _, item := range domains {
								if s, ok := item.(string); ok {
									analysis.RiskDomains = append(analysis.RiskDomains, s)
								}
							}
						}
						if proposal, ok := parsed["proposal_md"].(string); ok {
							analysis.ProposalMD = proposal
						}
						if specs, ok := parsed["specs_md"].(string); ok {
							analysis.SpecsMD = specs
						}
						if design, ok := parsed["design_md"].(string); ok {
							analysis.DesignMD = design
						}
						if tasks, ok := parsed["tasks_md"].(string); ok {
							analysis.TasksMD = tasks
						}
					}
				} else {
					analysis = deriveWorkflowAnalysis(task)
				}
			} else {
				analysis = deriveWorkflowAnalysis(task)
			}

			if analysis.Complexity == "" {
				analysis.Complexity = models.TaskComplexityEasy
			}

			// Generate and write actual OpenSpec files
			localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
			changeName := deriveChangeName(task)
			changeDir := filepath.Join(localPath, "openspec", "changes", changeName)
			if err := os.MkdirAll(changeDir, 0o755); err == nil {
				proposalContent := analysis.ProposalMD
				if proposalContent == "" {
					proposalContent = fmt.Sprintf("## Proposal for %s\n\n%s\n", task.Title, task.Description)
					analysis.ProposalMD = proposalContent
				}
				specsContent := analysis.SpecsMD
				if specsContent == "" {
					specsContent = fmt.Sprintf("## ADDED Requirements\n\n### Requirement: %s\n%s\n", task.Title, task.Description)
					analysis.SpecsMD = specsContent
				}
				designContent := analysis.DesignMD
				if designContent == "" {
					designContent = "## Design\n\nImplementation design details.\n"
					analysis.DesignMD = designContent
				}
				tasksContent := analysis.TasksMD
				if tasksContent == "" {
					var builder strings.Builder
					builder.WriteString("## Tasks\n\n")
					if len(analysis.ExecutionPlan) > 0 {
						for _, step := range analysis.ExecutionPlan {
							builder.WriteString(fmt.Sprintf("- [ ] %s\n", step))
						}
					} else {
						builder.WriteString("- [ ] Implement changes\n")
					}
					tasksContent = builder.String()
					analysis.TasksMD = tasksContent
				}
				_ = os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte(proposalContent), 0o644)
				_ = os.WriteFile(filepath.Join(changeDir, "specs.md"), []byte(specsContent), 0o644)
				_ = os.WriteFile(filepath.Join(changeDir, "design.md"), []byte(designContent), 0o644)
				_ = os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasksContent), 0o644)
				meta := fmt.Sprintf("changeName: %s\ntaskId: %s\nstatus: pending_review\n", changeName, task.ID)
				_ = os.WriteFile(filepath.Join(changeDir, ".openspec.yaml"), []byte(meta), 0o644)
			}

			raw, err := json.Marshal(analysis)
			if err != nil {
				return nil, err
			}
			var projectAutonomy string
			var projectReviewPolicy string
			if o.projects != nil {
				if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
					projectAutonomy = p.DefaultAutonomy
					projectReviewPolicy = p.AutoReviewPolicy
				}
			}
			specStatus, status := policy.ShouldAutoApproveSpec(
				analysis.Complexity,
				analysis.AffectedFiles,
				analysis.RiskDomains,
				agent.AutonomyLevel,
				projectAutonomy,
				projectReviewPolicy,
				len(analysis.ClarificationQuestions) > 0,
			)
			if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{
				Complexity: &analysis.Complexity,
				Analysis:   raw,
				SpecStatus: &specStatus,
			}); err != nil {
				return nil, err
			}
			if _, err := o.updateTaskStatus(ctx, task.ID, status); err != nil {
				return nil, err
			}
			task.Complexity = analysis.Complexity
			task.SpecStatus = specStatus
			task.Analysis = raw
			if specStatus == models.TaskSpecStatusPendingReview || specStatus == models.TaskSpecStatusChangesRequested {
				return nil, workflow.PauseError{Step: workflow.StepAnalyze, Reason: "workflow paused for human spec review"}
			}
			return map[string]any{"complexity": analysis.Complexity, "spec_status": specStatus}, nil
		},
		workflow.StepPlan: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			t, err := o.tasks.GetByID(ctx, task.ID)
			if err == nil && t.Complexity == models.TaskComplexityEasy {
				return map[string]any{"status": "skipped", "info": "skipped plan step for easy task"}, nil
			}
			var out map[string]any
			if o.llm != nil {
				out, err = o.runLLMStep(ctx, task, agent, jobID, workflow.StepPlan, "Create a concise JSON execution plan with subtasks, risks, and test strategy.")
			} else {
				plan := []any{
					map[string]any{"id": "backend", "role": models.AgentRoleBackend, "description": "Implement server-side changes and data contracts."},
					map[string]any{"id": "frontend", "role": models.AgentRoleFrontend, "description": "Implement user-facing workflow updates when applicable."},
				}
				out, err = map[string]any{"subtasks": plan}, nil
			}
			if err != nil {
				return nil, err
			}

			// Allocate branches idempotently for medium/hard tasks
			targetRepos, errRepos := o.loadTargetRepositories(ctx, task)
			if errRepos == nil && len(targetRepos) > 0 {
				integrationBranch := fmt.Sprintf("feature/%s", task.ID)
				beBranch := fmt.Sprintf("feature/%s-be", task.ID)
				feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

				ws, _ := o.LoadTaskWorkspace(ctx, task)
				o.setupRoleBranches(ctx, task, agent, jobID, targetRepos, ws)
				out["branches"] = map[string]string{
					"integration": integrationBranch,
					"backend":     beBranch,
					"frontend":    feBranch,
				}
			}

			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusCoding); err != nil {
				return nil, err
			}
			return out, nil
		},
		workflow.StepCodeBackend: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			backendAgent := agent
			if manager, ok := o.agents.(interface {
				AssignBackendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
			}); ok {
				if bg, err := manager.AssignBackendAgent(ctx, task); err == nil && bg != nil {
					backendAgent = bg
					o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned backend agent %s for backend coding step", backendAgent.Name))
				}
			}

			worktreeSuffix := ""
			t, _ := o.tasks.GetByID(ctx, task.ID)
			if t.Complexity != models.TaskComplexityEasy {
				worktreeSuffix = "-be-worktree"
				if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
					ws, _ := o.LoadTaskWorkspace(ctx, task)
					if err := o.setupRoleWorktrees(ctx, task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix); err != nil {
						return nil, err
					}
				}
			}

			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, backendAgent, jobID, workflow.StepCodeBackend, "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, backendAgent, workflow.StepCodeBackend, patch, worktreeSuffix); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, backendAgent, workflow.StepCodeBackend, worktreeSuffix); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "diff", diffText)
				}

				localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
				targetPath := localPath
				if worktreeSuffix != "" {
					targetPath = localPath + worktreeSuffix
				}

				changedFiles, diffErr := o.getChangedFiles(ctx, task, backendAgent, targetPath, worktreeSuffix)
				if diffErr != nil {
					o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
				}

				if worktreeSuffix != "" {
					if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
						ws, _ := o.LoadTaskWorkspace(ctx, task)
						o.commitRoleWorktrees(ctx, task, backendAgent, targetRepos, ws, "be", "backend", worktreeSuffix)
					}
				}

				if len(changedFiles) > 0 {
					if _, errT := o.runTargetedTests(ctx, task, backendAgent, jobID, "code_backend_test", changedFiles, worktreeSuffix); errT != nil {
						o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
					}
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepCodeFrontend: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			t, err := o.tasks.GetByID(ctx, task.ID)
			if err == nil {
				if t.Complexity == models.TaskComplexityEasy {
					return map[string]any{"status": "skipped", "info": "skipped frontend step for easy task"}, nil
				}
				var analysis models.TaskAnalysis
				if json.Unmarshal(t.Analysis, &analysis) == nil {
					hasFrontend := false
					for _, file := range analysis.AffectedFiles {
						if strings.HasPrefix(file, "web/") || strings.HasSuffix(file, ".tsx") || strings.HasSuffix(file, ".css") || strings.HasSuffix(file, ".html") {
							hasFrontend = true
							break
						}
					}
					if !hasFrontend {
						return map[string]any{"status": "skipped", "info": "no frontend files affected"}, nil
					}
				}
			}
			frontendAgent := agent
			if manager, ok := o.agents.(interface {
				AssignFrontendAgent(ctx context.Context, task *models.Task) (*models.Agent, error)
			}); ok {
				if fg, err := manager.AssignFrontendAgent(ctx, task); err == nil && fg != nil {
					frontendAgent = fg
					o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned frontend agent %s for frontend coding step", frontendAgent.Name))
				}
			}

			worktreeSuffix := ""
			if t != nil && t.Complexity != models.TaskComplexityEasy {
				worktreeSuffix = "-fe-worktree"
				if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
					ws, _ := o.LoadTaskWorkspace(ctx, task)
					if err := o.setupRoleWorktrees(ctx, task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix); err != nil {
						return nil, err
					}
				}
			}

			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, frontendAgent, jobID, workflow.StepCodeFrontend, "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, frontendAgent, workflow.StepCodeFrontend, patch, worktreeSuffix); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, frontendAgent, workflow.StepCodeFrontend, worktreeSuffix); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "diff", diffText)
				}

				localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
				targetPath := localPath
				if worktreeSuffix != "" {
					targetPath = localPath + worktreeSuffix
				}

				changedFiles, diffErr := o.getChangedFiles(ctx, task, frontendAgent, targetPath, worktreeSuffix)
				if diffErr != nil {
					o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
				}

				if worktreeSuffix != "" {
					if targetRepos, err := o.loadTargetRepositories(ctx, task); err == nil {
						ws, _ := o.LoadTaskWorkspace(ctx, task)
						o.commitRoleWorktrees(ctx, task, frontendAgent, targetRepos, ws, "fe", "frontend", worktreeSuffix)
					}
				}

				if len(changedFiles) > 0 {
					if _, errT := o.runTargetedTests(ctx, task, frontendAgent, jobID, "code_frontend_test", changedFiles, worktreeSuffix); errT != nil {
						o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
					}
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepMerge: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			t, err := o.tasks.GetByID(ctx, task.ID)
			if err == nil && t.Complexity == models.TaskComplexityEasy {
				if ws, errWS := o.LoadTaskWorkspace(ctx, task); errWS == nil {
					for i := range ws.Repos {
						ws.Repos[i].Status.MergeStatus = models.MergeStatusSkipped
					}
					_ = o.SaveTaskWorkspaceMetadata(task, ws)
				}
				return map[string]any{"status": "skipped", "info": "skipped merge step for easy task"}, nil
			}

			targetRepos, err := o.loadTargetRepositories(ctx, task)
			if err != nil {
				targetRepos = nil
			}

			ws, errWS := o.LoadTaskWorkspace(ctx, task)
			var workspace *models.TaskWorkspace
			if errWS == nil {
				workspace = ws
			}

			integrationBranch := fmt.Sprintf("feature/%s", task.ID)
			beBranch := fmt.Sprintf("feature/%s-be", task.ID)
			feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

			hasConflicts := false
			var conflictDetails []string

			for _, repo := range targetRepos {
				localPath := o.repoHostPath(task, workspace, repo)
				containerLocalPath := o.containerPathForHostPath(task, localPath, "")

				repoMergeStatus := models.MergeStatusMerged

				// 1. Checkout integration branch
				_, errCheckout := o.runSandboxStep(ctx, task, agent, "checkout_integration", fmt.Sprintf("git -C %s checkout %s", quoteShellArg(containerLocalPath), quoteShellArg(integrationBranch)))
				if errCheckout != nil {
					if errWS == nil {
						for i := range ws.Repos {
							if ws.Repos[i].RepoID == repo.ID {
								ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
							}
						}
						_ = o.SaveTaskWorkspaceMetadata(task, ws)
					}
					return nil, fmt.Errorf("checkout integration branch failed for repo %s: %w", repo.URL, errCheckout)
				}

				// Check if backend branch exists before merging
				hasBeBranch := false
				if _, errCheck := o.runSandboxStep(ctx, task, agent, "check_be_branch", fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/%s", quoteShellArg(containerLocalPath), quoteShellArg(beBranch))); errCheck == nil {
					hasBeBranch = true
				}
				if hasBeBranch {
					_, errMergeBe := o.runSandboxStep(ctx, task, agent, "merge_be", fmt.Sprintf("git -C %s merge --no-commit %s", quoteShellArg(containerLocalPath), quoteShellArg(beBranch)))
					if errMergeBe != nil {
						conflictCheck, errCC := o.runSandboxStep(ctx, task, agent, "conflict_check_be", fmt.Sprintf("git -C %s diff --name-only --diff-filter=U", quoteShellArg(containerLocalPath)))
						if errCC == nil {
							if stdout, ok := conflictCheck["stdout"].(string); ok && stdout != "" {
								hasConflicts = true
								repoMergeStatus = models.MergeStatusConflict
								conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (backend): %s", repo.URL, stdout))
							} else {
								repoMergeStatus = models.MergeStatusFailed
								if errWS == nil {
									for i := range ws.Repos {
										if ws.Repos[i].RepoID == repo.ID {
											ws.Repos[i].Status.MergeStatus = repoMergeStatus
										}
									}
									_ = o.SaveTaskWorkspaceMetadata(task, ws)
								}
								return nil, fmt.Errorf("merge backend branch failed for repo %s: %w", repo.URL, errMergeBe)
							}
						} else {
							repoMergeStatus = models.MergeStatusFailed
							if errWS == nil {
								for i := range ws.Repos {
									if ws.Repos[i].RepoID == repo.ID {
										ws.Repos[i].Status.MergeStatus = repoMergeStatus
									}
								}
								_ = o.SaveTaskWorkspaceMetadata(task, ws)
							}
							return nil, fmt.Errorf("merge backend branch failed for repo %s: %w", repo.URL, errMergeBe)
						}
					}
				}

				// Check if frontend branch exists before merging
				hasFeBranch := false
				if _, errCheck := o.runSandboxStep(ctx, task, agent, "check_fe_branch", fmt.Sprintf("git -C %s show-ref --verify --quiet refs/heads/%s", quoteShellArg(containerLocalPath), quoteShellArg(feBranch))); errCheck == nil {
					hasFeBranch = true
				}
				if hasFeBranch {
					_, errMergeFe := o.runSandboxStep(ctx, task, agent, "merge_fe", fmt.Sprintf("git -C %s merge --no-commit %s", quoteShellArg(containerLocalPath), quoteShellArg(feBranch)))
					if errMergeFe != nil {
						conflictCheck, errCC := o.runSandboxStep(ctx, task, agent, "conflict_check_fe", fmt.Sprintf("git -C %s diff --name-only --diff-filter=U", quoteShellArg(containerLocalPath)))
						if errCC == nil {
							if stdout, ok := conflictCheck["stdout"].(string); ok && stdout != "" {
								hasConflicts = true
								repoMergeStatus = models.MergeStatusConflict
								conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (frontend): %s", repo.URL, stdout))
							} else {
								repoMergeStatus = models.MergeStatusFailed
								if errWS == nil {
									for i := range ws.Repos {
										if ws.Repos[i].RepoID == repo.ID {
											ws.Repos[i].Status.MergeStatus = repoMergeStatus
										}
									}
									_ = o.SaveTaskWorkspaceMetadata(task, ws)
								}
								return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
							}
						} else {
							repoMergeStatus = models.MergeStatusFailed
							if errWS == nil {
								for i := range ws.Repos {
									if ws.Repos[i].RepoID == repo.ID {
										ws.Repos[i].Status.MergeStatus = repoMergeStatus
									}
								}
								_ = o.SaveTaskWorkspaceMetadata(task, ws)
							}
							return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
						}
					}
				}

				if !hasConflicts {
					// Commit the merge if there are staged changes
					commitMsg := "Merge role branches into integration"
					script := fmt.Sprintf("if ! git -C %[1]s diff --cached --quiet; then git -C %[1]s commit -m %[2]s; fi", quoteShellArg(containerLocalPath), quoteShellArg(commitMsg))
					if _, errCommit := o.runSandboxStep(ctx, task, agent, "commit_merge", script); errCommit != nil {
						if errWS == nil {
							for i := range ws.Repos {
								if ws.Repos[i].RepoID == repo.ID {
									ws.Repos[i].Status.MergeStatus = models.MergeStatusFailed
								}
							}
							_ = o.SaveTaskWorkspaceMetadata(task, ws)
						}
						return nil, fmt.Errorf("failed to commit merge for repo %s: %w", repo.URL, errCommit)
					}
				}

				if errWS == nil {
					for i := range ws.Repos {
						if ws.Repos[i].RepoID == repo.ID {
							ws.Repos[i].Status.MergeStatus = repoMergeStatus
						}
					}
				}
			}

			if errWS == nil {
				_ = o.SaveTaskWorkspaceMetadata(task, ws)
			}

			if hasConflicts {
				conflictStr := strings.Join(conflictDetails, "\n")
				_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepMerge, "conflict", conflictStr)
				return nil, workflow.PauseError{
					Step:   workflow.StepMerge,
					Reason: fmt.Sprintf("merge conflict in files:\n%s\n— manual resolution required", conflictStr),
				}
			}

			diffText, err := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepMerge, "")
			if err != nil {
				return nil, fmt.Errorf("merge check failed: %w", err)
			}
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
				return nil, err
			}
			return map[string]any{
				"status":    "changes_reconciled",
				"info":      "local changes reconciled",
				"diff_size": len(diffText),
			}, nil
		},
		workflow.StepReview: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			t, err := o.tasks.GetByID(ctx, task.ID)
			if err == nil && t.Complexity == models.TaskComplexityEasy {
				return map[string]any{"status": "skipped", "info": "skipped review step for easy task"}, nil
			}
			reviewerAgent := agent
			if manager, ok := o.agents.(interface {
				AssignReviewer(ctx context.Context, task *models.Task) (*models.Agent, error)
			}); ok {
				if rev, err := manager.AssignReviewer(ctx, task); err == nil && rev != nil {
					reviewerAgent = rev
					o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("assigned reviewer agent %s for review step", reviewerAgent.Name))
				}
			}

			// Enforce review-fix cycle limit.
			maxCycles := 3
			if o.projects != nil {
				if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.MaxReviewFixCycles > 0 {
					maxCycles = p.MaxReviewFixCycles
				}
			}
			reviewCycleCount := 0
			if checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID); cpErr == nil {
				for _, cp := range checkpoints {
					if cp.Step == workflow.StepReview {
						reviewCycleCount++
					}
				}
			}

			if o.llm != nil {
				diffText, _ := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepReview, "")
				instruction := "Review the proposed changes. Here is the current workspace diff:\n\n" + diffText + "\n\nReturn JSON findings with severity, file, line, and recommendation."
				out, err := o.runLLMStep(ctx, task, reviewerAgent, jobID, workflow.StepReview, instruction)
				if err != nil {
					return nil, err
				}
				hasFindings := true
				if parsed, ok := out["parsed"].(map[string]any); ok {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepReview, "review_findings", parsed)
					if findings, exists := parsed["findings"]; exists {
						if slice, ok := findings.([]any); ok && len(slice) == 0 {
							hasFindings = false
						}
					}
				}
				nextStatus := models.TaskStatusFixing
				if !hasFindings {
					nextStatus = models.TaskStatusTesting
				}
				// If we've exceeded the cycle limit, skip fix and proceed to test.
				if hasFindings && reviewCycleCount >= maxCycles {
					o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("review-fix cycle limit reached (%d/%d), proceeding to test despite findings", reviewCycleCount, maxCycles))
					nextStatus = models.TaskStatusTesting
					out["cycle_limit_reached"] = true
				}
				if _, err := o.updateTaskStatus(ctx, task.ID, nextStatus); err != nil {
					return nil, err
				}
				return out, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepFix: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			t, err := o.tasks.GetByID(ctx, task.ID)
			if err == nil && t.Complexity == models.TaskComplexityEasy {
				return map[string]any{"status": "skipped", "info": "skipped fix step for easy task"}, nil
			}

			var prFeedback string
			if checkpoints, cpErr := o.workflows.ListCheckpoints(ctx, task.ID); cpErr == nil {
				for _, cp := range checkpoints {
					if cp.Step == "pr_rejection" {
						var state map[string]any
						if json.Unmarshal(cp.State, &state) == nil {
							if f, _ := state["feedback"].(string); f != "" {
								prFeedback = f
							}
						}
					}
				}
			}

			if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
				if limitReached, _ := reviewOut["cycle_limit_reached"].(bool); limitReached {
					return map[string]any{
						"status": "skipped",
						"info":   "review-fix cycle limit reached, skipping fix step",
					}, nil
				}
				if prFeedback == "" {
					if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
						if findings, exists := parsed["findings"]; exists {
							if slice, ok := findings.([]any); ok && len(slice) == 0 {
								return map[string]any{
									"status": "skipped",
									"info":   "no review findings, skipped fix step",
								}, nil
							}
						}
					}
				}
			}
			if o.llm != nil {
				instruction := "Fix review findings. Return JSON with fixes_applied, files_changed, and patch text when available."
				if prFeedback != "" {
					instruction = fmt.Sprintf("Fix review findings and address the following PR rejection feedback:\n\n%s\n\nReturn JSON with fixes_applied, files_changed, and patch text when available.", prFeedback)
				}

				out, err := o.runLLMStep(ctx, task, agent, jobID, workflow.StepFix, instruction)
				if err != nil {
					return nil, err
				}
				var patchApplied bool
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, agent, workflow.StepFix, patch, ""); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
						patchApplied = true
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepFix, ""); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "diff", diffText)
				}

				if patchApplied {
					localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)

					changedFiles, diffErr := o.getChangedFiles(ctx, task, agent, localPath, "")
					if diffErr != nil {
						o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to get changed files: %v", diffErr))
					}
					if len(changedFiles) > 0 {
						if _, errT := o.runTargetedTests(ctx, task, agent, jobID, "fix_test", changedFiles, ""); errT != nil {
							o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests failed: %v", errT))
						}
					}

					if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
						return nil, err
					}
					// We don't delete review & fix checkpoints here anymore; they are skipped when resuming
					// using the job.Step filter in orchestrator_worker.go to preserve cycle counts in DB.
					return nil, workflow.ErrReviewFixLoop
				}

				return map[string]any{
					"status": "success",
					"info":   "no fixes applied",
				}, nil
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepTest: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting); err != nil {
				return nil, err
			}
			script := `
run_verification() {
	local dir="$1"
	echo "Verifying repository in $dir..."
	cd "$dir" || return 1

	if [ -f go.mod ]; then
		go test ./... || return 1
	elif [ -f package.json ]; then
		npm test || return 1
	fi

	local lint_ran=0
	if [ -f .golangci.yml ] && command -v golangci-lint >/dev/null 2>&1; then
		golangci-lint run || return 1
		lint_ran=1
	fi
	if [ -f package.json ] && grep -q '"lint"' package.json; then
		npm run lint || return 1
		lint_ran=1
	fi
	if [ $lint_ran -eq 1 ]; then
		echo "LINT_STATUS: PASSED"
	else
		echo "LINT_STATUS: NOT_CONFIGURED"
	fi

	local build_ran=0
	if [ -f go.mod ]; then
		go build ./... || return 1
		build_ran=1
	fi
	if [ -f package.json ] && grep -q '"build"' package.json; then
		npm run build || return 1
		build_ran=1
	fi
	if [ $build_ran -eq 1 ]; then
		echo "BUILD_STATUS: PASSED"
	else
		echo "BUILD_STATUS: NOT_CONFIGURED"
	fi
}

found_repos=0
for d in code/repos/*/main ; do
	if [ -d "$d" ]; then
		(run_verification "$d") || exit 1
		found_repos=1
	fi
done

if [ $found_repos -eq 0 ]; then
	if [ -d .git ] || [ -f go.mod ] || [ -f package.json ]; then
		run_verification "." || exit 1
	else
		for d in */ ; do
			d_clean="${d%/}"
			if [ -d "$d_clean/.git" ] || [ -f "$d_clean/go.mod" ] || [ -f "$d_clean/package.json" ]; then
				(run_verification "$d_clean") || exit 1
			fi
		done
	fi
fi
`
			out, err := o.runSandboxStep(ctx, task, agent, workflow.StepTest, script)
			if err != nil {
				if ws, errWS := o.LoadTaskWorkspace(ctx, task); errWS == nil {
					for i := range ws.Repos {
						ws.Repos[i].Status.TestStatus = models.TestStatusFailed
					}
					_ = o.SaveTaskWorkspaceMetadata(task, ws)
				}
				return nil, err
			}

			if ws, errWS := o.LoadTaskWorkspace(ctx, task); errWS == nil {
				for i := range ws.Repos {
					ws.Repos[i].Status.TestStatus = models.TestStatusPassed
				}
				_ = o.SaveTaskWorkspaceMetadata(task, ws)
			}

			stdout, _ := out["stdout"].(string)

			lintStatus := "not_configured"
			if strings.Contains(stdout, "LINT_STATUS: PASSED") {
				lintStatus = "passed"
			}

			buildStatus := "not_configured"
			if strings.Contains(stdout, "BUILD_STATUS: PASSED") {
				buildStatus = "passed"
			}

			out["exit_code"] = 0
			out["passed"] = true
			out["lint_status"] = lintStatus
			out["build_status"] = buildStatus
			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepTest, "test_output", out)
			return out, nil
		},
		workflow.StepPR: func(ctx context.Context, stepCtx workflow.StepContext) (map[string]any, error) {
			if o.gitOps == nil {
				return nil, fmt.Errorf("gitops client is not configured")
			}
			targetRepos, err := o.loadTargetRepositories(ctx, task)
			if err != nil {
				return nil, fmt.Errorf("list project repositories: %w", err)
			}
			if len(targetRepos) == 0 {
				return nil, fmt.Errorf("no repository linked to project %s", task.ProjectID)
			}

			// Capture overall workspace diff to find changed files
			_, _ = o.captureWorkspaceDiff(ctx, task, agent, workflow.StepPR, "")

			// Get test results from previous step if available
			var testOut map[string]any
			if stepCtx.Inputs != nil {
				testOut = stepCtx.Inputs[workflow.StepTest]
			}

			var targetedCodeBackendPassed bool
			var targetedCodeBackendRun bool
			var targetedCodeFrontendPassed bool
			var targetedCodeFrontendRun bool
			var targetedFixPassed bool
			var targetedFixRun bool

			if o.artifacts != nil {
				if arts, err := o.artifacts.ListByJobID(ctx, jobID); err == nil {
					for _, art := range arts {
						if art.Type == "targeted_test" {
							var payload map[string]any
							if json.Unmarshal(art.Payload, &payload) == nil {
								status, _ := payload["status"].(string)
								if art.Step == "code_backend_test" {
									targetedCodeBackendRun = true
									if status == "passed" {
										targetedCodeBackendPassed = true
									}
								} else if art.Step == "code_frontend_test" {
									targetedCodeFrontendRun = true
									if status == "passed" {
										targetedCodeFrontendPassed = true
									}
								} else if art.Step == "fix_test" {
									targetedFixRun = true
									if status == "passed" {
										targetedFixPassed = true
									}
								}
							}
						}
					}
				}
			}

			testOutCopy := map[string]any{}
			for k, v := range testOut {
				testOutCopy[k] = v
			}
			if targetedCodeBackendRun {
				testOutCopy["targeted_code_backend_passed"] = targetedCodeBackendPassed
			}
			if targetedCodeFrontendRun {
				testOutCopy["targeted_code_frontend_passed"] = targetedCodeFrontendPassed
			}
			if targetedFixRun {
				testOutCopy["targeted_fix_passed"] = targetedFixPassed
			}

			var createdPRs []string
			var createdBranches []string
			var createdPRSummaries []models.PRSummary

			ws, errWS := o.LoadTaskWorkspace(ctx, task)
			var workspace *models.TaskWorkspace
			if errWS == nil {
				workspace = ws
			}

			for _, repo := range targetRepos {
				localPath := o.repoHostPath(task, workspace, repo)

				// Check if this specific repo has changes before creating branch
				// For simplicity, we create branch anyway, if no changes commit will fail or we can skip.
				// A better way is to run `git status --porcelain` in localPath.
				containerLocalPath := o.containerPathForHostPath(task, localPath, "")
				out, errSandbox := o.runSandboxStep(ctx, task, agent, "git_status_"+repo.ID, fmt.Sprintf("cd %s && git status --porcelain", quoteShellArg(containerLocalPath)))
				if errSandbox != nil {
					continue // directory might not exist or not a git repo
				}
				statusOut, _ := out["stdout"].(string)
				if strings.TrimSpace(statusOut) == "" && task.Complexity == models.TaskComplexityEasy {
					continue // no changes in this repo for easy tasks
				}

				branchName := fmt.Sprintf("feature/%s", task.ID)
				if _, err := o.runSandboxStep(ctx, task, agent, "checkout_pr_branch", fmt.Sprintf("git -C %s checkout -B %s", quoteShellArg(containerLocalPath), quoteShellArg(branchName))); err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("checkout branch failed for %s: %v", repo.URL, err))
					continue
				}

				// Extract changed files for this repo from statusOut
				var repoChangedFiles []string
				for _, line := range strings.Split(statusOut, "\n") {
					line = strings.TrimSpace(line)
					if len(line) > 2 {
						repoChangedFiles = append(repoChangedFiles, strings.TrimSpace(line[2:]))
					}
				}

				commitMsg := fmt.Sprintf("AutoCodeOS: implement task %s\n\nTitle: %s", task.ID, task.Title)
				if err := o.gitOps.CommitAndPush(ctx, localPath, repo.URL, branchName, commitMsg, nil, agent.Role); err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("commit and push failed for %s: %v", repo.URL, err))
					continue
				}

				// Capture this specific repo's diff
				diffOut, _ := o.runSandboxStep(ctx, task, agent, "git_diff_"+repo.ID, fmt.Sprintf("cd %s && git diff main...HEAD", quoteShellArg(containerLocalPath)))
				repoDiffText, _ := diffOut["stdout"].(string)
				if repoDiffText == "" {
					// Fallback to git diff
					diffOut, _ = o.runSandboxStep(ctx, task, agent, "git_diff_fallback_"+repo.ID, fmt.Sprintf("cd %s && git diff", quoteShellArg(containerLocalPath)))
					repoDiffText, _ = diffOut["stdout"].(string)
				}

				prGen := NewPRGenerator()
				summary := prGen.GenerateSummary(ctx, task, agent, repoChangedFiles, repoDiffText, testOutCopy)

				maxReviewFixCycles := 3
				if o.projects != nil {
					if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.MaxReviewFixCycles > 0 {
						maxReviewFixCycles = p.MaxReviewFixCycles
					}
				}
				checkpoints, _ := o.workflows.ListCheckpoints(ctx, task.ID)
				rejectionCount := 0
				for _, cp := range checkpoints {
					if cp.Step == "pr_rejection" {
						rejectionCount++
					}
				}
				summary.ReviewLimitExceeded = (rejectionCount >= maxReviewFixCycles-1)

				prURL, err := o.gitOps.CreatePullRequest(ctx, repo.URL, branchName, summary.Title, summary.Body)
				if err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("create PR failed for %s: %v", repo.URL, err))
					continue
				}
				if strings.TrimSpace(prURL) == "" {
					o.log(ctx, task.ID, nil, "info", fmt.Sprintf("create PR returned no URL for %s; skipping", repo.URL))
					continue
				}

				createdPRs = append(createdPRs, prURL)
				createdBranches = append(createdBranches, branchName)
				summary.PRURL = prURL
				createdPRSummaries = append(createdPRSummaries, *summary)

			}

			if len(createdPRs) > 0 {
				pqCreatedPRs := pq.StringArray(createdPRs)
				prMetadataRaw, _ := json.Marshal(createdPRSummaries)
				if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{
					PRURLs:     &pqCreatedPRs,
					PRMetadata: prMetadataRaw,
				}); err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save PR metadata: %v", err))
				}
			}

			status := models.TaskStatusPrReady
			if len(createdPRs) == 0 {
				status = models.TaskStatusPrReady
				noChangesSummaries := []models.PRSummary{{
					Title:  "No changes detected",
					Body:   "No code modifications were required.",
					Status: "no_changes",
				}}
				noChangesMetadata, _ := json.Marshal(noChangesSummaries)
				if _, err := o.tasks.Update(ctx, task.ID, models.UpdateTaskInput{
					PRMetadata: noChangesMetadata,
				}); err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save PR metadata for no-changes: %v", err))
				}
			}
			if _, err := o.updateTaskStatus(ctx, task.ID, status); err != nil {
				return nil, err
			}

			if len(createdPRs) == 0 {
				return map[string]any{
					"status":   "no_changes_detected",
					"branches": createdBranches,
					"pr_urls":  createdPRs,
				}, nil
			}

			return map[string]any{
				"status":   "pr_ready_for_human_approval",
				"branches": createdBranches,
				"pr_urls":  createdPRs,
			}, nil
		},
	}

	for stepID, runner := range runners {
		runners[stepID] = o.withCheckpointRecovery(task, agent, stepID, runner)
	}
	return runners
}
