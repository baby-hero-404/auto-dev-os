package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"regexp"

	"github.com/auto-code-os/auto-code-os/server/internal/policy"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel"
)

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string) map[string]workflow.StepFunc {
	runners := map[string]workflow.StepFunc{
		workflow.StepContextLoad: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusContextLoading); err != nil {
				return nil, err
			}

			localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
			repoPaths := []string{localPath}
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
			repos, errRepos := o.repositories.ListByProjectID(ctx, task.ProjectID)
			if errRepos == nil && len(repos) > 0 {
				var targetRepos []models.Repository
				if task.RepositoryID != nil {
					for _, r := range repos {
						if r.ID == *task.RepositoryID {
							targetRepos = append(targetRepos, r)
							break
						}
					}
				} else {
					targetRepos = repos
				}

				integrationBranch := fmt.Sprintf("feature/%s", task.ID)
				beBranch := fmt.Sprintf("feature/%s-be", task.ID)
				feBranch := fmt.Sprintf("feature/%s-fe", task.ID)

				for _, repo := range targetRepos {
					localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
					if task.RepositoryID == nil {
						parts := strings.Split(repo.URL, "/")
						repoName := parts[len(parts)-1]
						repoName = strings.TrimSuffix(repoName, ".git")
						localPath = filepath.Join(localPath, repoName)
					}

					containerLocalPath := o.containerPathForHostPath(task, localPath, "")
					script := fmt.Sprintf(`
set -e
git -C %[1]s show-ref --verify --quiet refs/heads/%[2]s || git -C %[1]s branch %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[1]s branch %[3]s %[2]s
git -C %[1]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[1]s branch %[4]s %[2]s
`, quoteShellArg(containerLocalPath), quoteShellArg(integrationBranch), quoteShellArg(beBranch), quoteShellArg(feBranch))

					if _, errSandbox := o.runSandboxStep(ctx, task, agent, "create_role_branches", script); errSandbox != nil {
						o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("failed to create role branches for %s: %v", repo.URL, errSandbox))
					}
				}
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
				if repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID); err == nil {
					var targetRepos []models.Repository
					if task.RepositoryID != nil {
						for _, r := range repos {
							if r.ID == *task.RepositoryID {
								targetRepos = append(targetRepos, r)
								break
							}
						}
					} else {
						targetRepos = repos
					}
					for _, repo := range targetRepos {
						localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
						if task.RepositoryID == nil {
							parts := strings.Split(repo.URL, "/")
							repoName := parts[len(parts)-1]
							repoName = strings.TrimSuffix(repoName, ".git")
							localPath = filepath.Join(localPath, repoName)
						}
						bePath := o.hostWorktreePath(task, localPath, worktreeSuffix)
						beBranch := fmt.Sprintf("feature/%s-be", task.ID)
						integrationBranch := fmt.Sprintf("feature/%s", task.ID)
						containerBePath := o.containerPathForHostPath(task, bePath, "")
						containerLocalPath := o.containerPathForHostPath(task, localPath, "")
						script := fmt.Sprintf(`
set -e
git -C %[2]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[2]s branch %[4]s
git -C %[2]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[2]s branch %[3]s %[4]s
if [ -d %[1]s ] && grep -q '^gitdir:' %[1]s/.git 2>/dev/null; then
	echo 'worktree valid'
else
	rm -rf %[1]s
	git -C %[2]s worktree add %[1]s %[3]s
fi
`,
							quoteShellArg(containerBePath),
							quoteShellArg(containerLocalPath),
							quoteShellArg(beBranch),
							quoteShellArg(integrationBranch),
						)
						if _, errWT := o.runSandboxStep(ctx, task, backendAgent, "worktree_be", script); errWT != nil {
							return nil, fmt.Errorf("failed to setup backend worktree for repo %s: %w", repo.URL, errWT)
						}
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
					// Commit changes to role branch
					repos, _ := o.repositories.ListByProjectID(ctx, task.ProjectID)
					var targetRepos []models.Repository
					if task.RepositoryID != nil {
						for _, r := range repos {
							if r.ID == *task.RepositoryID {
								targetRepos = append(targetRepos, r)
								break
							}
						}
					} else {
						targetRepos = repos
					}
					commitMsg := fmt.Sprintf("AutoCodeOS [backend]: %s", task.Title)
					for _, repo := range targetRepos {
						localRepoPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
						if task.RepositoryID == nil {
							parts := strings.Split(repo.URL, "/")
							repoName := parts[len(parts)-1]
							repoName = strings.TrimSuffix(repoName, ".git")
							localRepoPath = filepath.Join(localRepoPath, repoName)
						}
						bePath := localRepoPath + worktreeSuffix
						containerBePath := o.containerPathForHostPath(task, bePath, worktreeSuffix)
						script := fmt.Sprintf("git -C %[1]s add . && git -C %[1]s commit -m %[2]s || true",
							quoteShellArg(containerBePath),
							quoteShellArg(commitMsg),
						)
						_, _ = o.runSandboxStepInWorktree(ctx, task, backendAgent, "commit_be", script, worktreeSuffix)
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
				if repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID); err == nil {
					var targetRepos []models.Repository
					if task.RepositoryID != nil {
						for _, r := range repos {
							if r.ID == *task.RepositoryID {
								targetRepos = append(targetRepos, r)
								break
							}
						}
					} else {
						targetRepos = repos
					}
					for _, repo := range targetRepos {
						localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
						if task.RepositoryID == nil {
							parts := strings.Split(repo.URL, "/")
							repoName := parts[len(parts)-1]
							repoName = strings.TrimSuffix(repoName, ".git")
							localPath = filepath.Join(localPath, repoName)
						}
						fePath := o.hostWorktreePath(task, localPath, worktreeSuffix)
						feBranch := fmt.Sprintf("feature/%s-fe", task.ID)
						integrationBranch := fmt.Sprintf("feature/%s", task.ID)
						containerFePath := o.containerPathForHostPath(task, fePath, "")
						containerLocalPath := o.containerPathForHostPath(task, localPath, "")
						script := fmt.Sprintf(`
set -e
git -C %[2]s show-ref --verify --quiet refs/heads/%[4]s || git -C %[2]s branch %[4]s
git -C %[2]s show-ref --verify --quiet refs/heads/%[3]s || git -C %[2]s branch %[3]s %[4]s
if [ -d %[1]s ] && grep -q '^gitdir:' %[1]s/.git 2>/dev/null; then
	echo 'worktree valid'
else
	rm -rf %[1]s
	git -C %[2]s worktree add %[1]s %[3]s
fi
`,
							quoteShellArg(containerFePath),
							quoteShellArg(containerLocalPath),
							quoteShellArg(feBranch),
							quoteShellArg(integrationBranch),
						)
						if _, errWT := o.runSandboxStep(ctx, task, frontendAgent, "worktree_fe", script); errWT != nil {
							return nil, fmt.Errorf("failed to setup frontend worktree for repo %s: %w", repo.URL, errWT)
						}
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
					// Commit changes to role branch
					repos, _ := o.repositories.ListByProjectID(ctx, task.ProjectID)
					var targetRepos []models.Repository
					if task.RepositoryID != nil {
						for _, r := range repos {
							if r.ID == *task.RepositoryID {
								targetRepos = append(targetRepos, r)
								break
							}
						}
					} else {
						targetRepos = repos
					}
					commitMsg := fmt.Sprintf("AutoCodeOS [frontend]: %s", task.Title)
					for _, repo := range targetRepos {
						localRepoPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
						if task.RepositoryID == nil {
							parts := strings.Split(repo.URL, "/")
							repoName := parts[len(parts)-1]
							repoName = strings.TrimSuffix(repoName, ".git")
							localRepoPath = filepath.Join(localRepoPath, repoName)
						}
						fePath := localRepoPath + worktreeSuffix
						containerFePath := o.containerPathForHostPath(task, fePath, worktreeSuffix)
						script := fmt.Sprintf("git -C %[1]s add . && git -C %[1]s commit -m %[2]s || true",
							quoteShellArg(containerFePath),
							quoteShellArg(commitMsg),
						)
						_, _ = o.runSandboxStepInWorktree(ctx, task, frontendAgent, "commit_fe", script, worktreeSuffix)
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
				return map[string]any{"status": "skipped", "info": "skipped merge step for easy task"}, nil
			}

			repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
			var targetRepos []models.Repository
			if err == nil && len(repos) > 0 {
				if task.RepositoryID != nil {
					for _, r := range repos {
						if r.ID == *task.RepositoryID {
							targetRepos = append(targetRepos, r)
							break
						}
					}
				} else {
					targetRepos = repos
				}
			}

			integrationBranch := fmt.Sprintf("feature/%s", task.ID)
			beBranch := fmt.Sprintf("feature/%s-be", task.ID)
			feBranch := fmt.Sprintf("feature/%s-fe", task.ID)
			
			hasConflicts := false
			var conflictDetails []string

			for _, repo := range targetRepos {
				localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
				if task.RepositoryID == nil {
					parts := strings.Split(repo.URL, "/")
					repoName := parts[len(parts)-1]
					repoName = strings.TrimSuffix(repoName, ".git")
					localPath = filepath.Join(localPath, repoName)
				}
				containerLocalPath := o.containerPathForHostPath(task, localPath, "")
				// 1. Checkout integration branch
				_, errCheckout := o.runSandboxStep(ctx, task, agent, "checkout_integration", fmt.Sprintf("git -C %s checkout %s", quoteShellArg(containerLocalPath), quoteShellArg(integrationBranch)))
				if errCheckout != nil {
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
								conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (backend): %s", repo.URL, stdout))
							} else {
								return nil, fmt.Errorf("merge backend branch failed for repo %s: %w", repo.URL, errMergeBe)
							}
						} else {
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
								conflictDetails = append(conflictDetails, fmt.Sprintf("Repo %s (frontend): %s", repo.URL, stdout))
							} else {
								return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
							}
						} else {
							return nil, fmt.Errorf("merge frontend branch failed for repo %s: %w", repo.URL, errMergeFe)
						}
					}
				}

				if !hasConflicts {
					// Commit the merge if there are staged changes
					commitMsg := "Merge role branches into integration"
					script := fmt.Sprintf("if ! git -C %[1]s diff --cached --quiet; then git -C %[1]s commit -m %[2]s; fi", quoteShellArg(containerLocalPath), quoteShellArg(commitMsg))
					if _, errCommit := o.runSandboxStep(ctx, task, agent, "commit_merge", script); errCommit != nil {
						return nil, fmt.Errorf("failed to commit merge for repo %s: %w", repo.URL, errCommit)
					}
				}
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
`
			out, err := o.runSandboxStep(ctx, task, agent, workflow.StepTest, script)
			if err != nil {
				return nil, err
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
			autonomy := agent.AutonomyLevel
			if o.projects != nil {
				if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.DefaultAutonomy != "" && autonomy == "" {
					autonomy = p.DefaultAutonomy
				}
			}

			if o.gitOps == nil {
				return nil, fmt.Errorf("gitops client is not configured")
			}
			repos, err := o.repositories.ListByProjectID(ctx, task.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("list project repositories: %w", err)
			}
			if len(repos) == 0 {
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
			if testOut != nil {
				for k, v := range testOut {
					testOutCopy[k] = v
				}
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

			var targetRepos []models.Repository
			if task.RepositoryID != nil {
				for _, r := range repos {
					if r.ID == *task.RepositoryID {
						targetRepos = append(targetRepos, r)
						break
					}
				}
				if len(targetRepos) == 0 {
					return nil, fmt.Errorf("task repository %s not found", *task.RepositoryID)
				}
			} else {
				targetRepos = repos
			}

			var createdPRs []string
			var createdBranches []string
			var allChangedFiles []string
			var createdPRSummaries []models.PRSummary

			for _, repo := range targetRepos {
				localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
				if task.RepositoryID == nil {
					parts := strings.Split(repo.URL, "/")
					repoName := parts[len(parts)-1]
					repoName = strings.TrimSuffix(repoName, ".git")
					localPath = filepath.Join(localPath, repoName)
				}

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
				allChangedFiles = append(allChangedFiles, repoChangedFiles...)

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
				summary.ReviewLimitExceeded = (rejectionCount >= maxReviewFixCycles - 1)

				prURL, err := o.gitOps.CreatePullRequest(ctx, repo.URL, branchName, summary.Title, summary.Body)
				if err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("create PR failed for %s: %v", repo.URL, err))
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
				status = models.TaskStatusMerged
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

func (o *Orchestrator) getSuccessfulCheckpoint(ctx context.Context, taskID string, step string) (map[string]any, bool) {
	checkpoints, err := o.workflows.ListCheckpoints(ctx, taskID)
	if err != nil {
		return nil, false
	}
	var latestSuccess *models.WorkflowCheckpoint
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step == step {
			var state map[string]any
			if err := json.Unmarshal(cp.State, &state); err == nil {
				if state["status"] == "success" {
					latestSuccess = &cp
					break
				}
			}
		}
	}
	if latestSuccess != nil {
		var state map[string]any
		_ = json.Unmarshal(latestSuccess.State, &state)
		if out, ok := state["output"].(map[string]any); ok {
			return out, true
		}
		return map[string]any{}, true
	}
	return nil, false
}

func (o *Orchestrator) getSavedPatch(ctx context.Context, taskID string, step string) (string, error) {
	if o.artifacts == nil {
		return "", fmt.Errorf("artifacts repository is not configured")
	}
	arts, err := o.artifacts.ListByTaskID(ctx, taskID)
	if err != nil {
		return "", err
	}
	var latestPatch *models.WorkflowArtifact
	for i := len(arts) - 1; i >= 0; i-- {
		art := arts[i]
		if art.Step == step && art.Type == "patch" {
			latestPatch = &art
			break
		}
	}
	if latestPatch == nil {
		return "", fmt.Errorf("no patch artifact found for step %s", step)
	}
	var patch string
	if err := json.Unmarshal(latestPatch.Payload, &patch); err == nil {
		return patch, nil
	}
	return string(latestPatch.Payload), nil
}

func (o *Orchestrator) withCheckpointRecovery(task *models.Task, agent *models.Agent, stepID string, runner workflow.StepFunc) workflow.StepFunc {
	return func(ctx context.Context, sc workflow.StepContext) (map[string]any, error) {
		if stepID != workflow.StepAnalyze {
			if output, exists := o.getSuccessfulCheckpoint(ctx, task.ID, stepID); exists {
				o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: resuming from previous successful checkpoint", stepID))

				if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix {
					if patch, err := o.getSavedPatch(ctx, task.ID, stepID); err == nil && patch != "" {
						o.log(ctx, task.ID, nil, "info", fmt.Sprintf("step %s: re-applying saved patch to workspace", stepID))
						
						worktreeSuffix := ""
						if stepID == workflow.StepCodeBackend {
							worktreeSuffix = "-be-worktree"
						} else if stepID == workflow.StepCodeFrontend {
							worktreeSuffix = "-fe-worktree"
						}

						if applyErr := o.applyPatch(ctx, task, agent, stepID, patch, worktreeSuffix); applyErr != nil {
							o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("step %s: failed to re-apply patch (%v), rerunning step", stepID, applyErr))
							return runner(ctx, sc)
						}
					}
				}

				switch stepID {
				case workflow.StepPlan:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusCoding)
				case workflow.StepMerge:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
				case workflow.StepReview:
					nextStatus := models.TaskStatusTesting
					if parsed, ok := output["parsed"].(map[string]any); ok {
						if findings, exists := parsed["findings"]; exists {
							if slice, ok := findings.([]any); ok && len(slice) > 0 {
								nextStatus = models.TaskStatusFixing
							}
						}
					}
					_, _ = o.updateTaskStatus(ctx, task.ID, nextStatus)
				case workflow.StepFix:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing)
				case workflow.StepTest:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting)
				case workflow.StepPR:
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusHumanReview)
				}

				return output, nil
			}
		}
		return runner(ctx, sc)
	}
}

func (o *Orchestrator) runLLMStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string) (map[string]any, error) {
	if o.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	var messages []llm.Message
	var err error
	if o.prompts != nil {
		messages, _, err = o.prompts.AssembleForAgent(ctx, *task, agent, nil)
		if err != nil {
			return nil, err
		}
	} else {
		messages = []llm.Message{{Role: "user", Content: task.Title + "\n\n" + task.Description}}
	}
	fullInstruction := instruction
	
	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}
	if len(analysis.AffectedFiles) > 0 && (stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix || stepID == workflow.StepReview) {
		var b strings.Builder
		b.WriteString("\n\n### Workspace Affected Files ###\n")
		localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
		for _, file := range analysis.AffectedFiles {
			content, err := os.ReadFile(filepath.Join(localPath, file))
			if err == nil {
				b.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", file, string(content)))
			}
		}
		fullInstruction += b.String()
	}

	if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix || stepID == workflow.StepPlan || stepID == workflow.StepAnalyze {
		fullInstruction += "\n\nCRITICAL REQUIREMENT: Do NOT output any tool calls, function calls, or markdown block thoughts. You do NOT have tool execution capabilities in this single-shot step. You MUST output ONLY a valid JSON object matching the requested format directly (or inside a ```json ``` block)."
	}
	if stepID == workflow.StepCodeBackend || stepID == workflow.StepCodeFrontend || stepID == workflow.StepFix {
		fullInstruction += "\n\nCRITICAL REQUIREMENT: The patch/diff field MUST contain a valid Unified Git Diff (starting with 'diff --git') representing all source code changes. Do NOT output raw file contents. Do NOT include any text outside the JSON structure."
	}
	messages = append(messages, llm.Message{Role: "user", Content: "Workflow step: " + stepID + "\n\n" + fullInstruction})

	// Save prompt artifact
	_ = o.saveArtifact(ctx, jobID, task.ID, stepID, "prompt", messages)

	routeName := agent.ModelLevelGroup
	if o.projects != nil {
		if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
			if agent.Role == models.AgentRolePlanner && p.DefaultModelLevel != "" {
				// Spec: DefaultModelLevel is specifically the default for the Planner agent
				routeName = p.DefaultModelLevel
			} else if (routeName == "" || routeName == "default") && p.DefaultModelLevel != "" {
				// For other agents, act as fallback
				routeName = p.DefaultModelLevel
			}
		}
	}

	ctx = llm.WithRouteOptions(ctx, llm.RouteOptions{
		Complexity: task.Complexity,
		OrgID:      agent.OrgID,
		ProjectID:  task.ProjectID,
		AgentID:    agent.ID,
		TaskID:     task.ID,
		RouteName:  routeName,
	})
	resp, err := o.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}
	o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: llm response from %s", stepID, resp.Model))
	var parsed map[string]any
	if parsedJSON, err := parseJSONMarkdown(resp.Content); err == nil {
		parsed = parsedJSON
	} else {
		parsed = map[string]any{"raw_content": resp.Content}
	}

	// Save llm_response artifact
	_ = o.saveArtifact(ctx, jobID, task.ID, stepID, "llm_response", parsed)

	return map[string]any{
		"status":        "llm_completed",
		"model":         resp.Model,
		"content":       resp.Content,
		"parsed":        parsed,
		"prompt_tokens": resp.PromptTokens,
		"output_tokens": resp.OutputTokens,
	}, nil
}

func (o *Orchestrator) runSandboxStep(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string) (map[string]any, error) {
	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
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
		AffectedFiles: []string{},
		Risks:         []string{"Workflow uses deterministic planning until full LLM step execution is enabled."},
		ExecutionPlan: []string{
			"Assemble prompt with role, rules, and retrieved context.",
			"Decompose work into typed subtasks.",
			"Run backend and frontend coding tracks in parallel sandboxes.",
			"Merge, review, fix, test, and prepare PR approval checkpoint.",
		},
		ClarificationQuestions: questions,
	}
}

func parseJSONMarkdown(content string) (map[string]any, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if strings.HasSuffix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(trimmed), &res); err != nil {
		start := strings.Index(trimmed, "{")
		end := strings.LastIndex(trimmed, "}")
		if start != -1 && end != -1 && end > start {
			trimmed = trimmed[start : end+1]
			if err := json.Unmarshal([]byte(trimmed), &res); err == nil {
				return res, nil
			}
		}
		return nil, err
	}
	return res, nil
}

func (o *Orchestrator) applyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string, worktreeSuffix string) error {
	if patchText == "" {
		return nil
	}

	// Scan lines of patch to extract modified files
	lines := strings.Split(patchText, "\n")
	var modifiedFiles []string
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			file := strings.TrimPrefix(line, "+++ b/")
			file = strings.TrimSpace(file)
			if file != "/dev/null" {
				modifiedFiles = append(modifiedFiles, file)
			}
		} else if strings.HasPrefix(line, "--- a/") {
			file := strings.TrimPrefix(line, "--- a/")
			file = strings.TrimSpace(file)
			if file != "/dev/null" {
				modifiedFiles = append(modifiedFiles, file)
			}
		}
	}

	// Enforce affected files if specified
	if task.Analysis != nil {
		var analysis models.TaskAnalysis
		if err := json.Unmarshal(task.Analysis, &analysis); err == nil && len(analysis.AffectedFiles) > 0 {
			// Validate all files modified in the patch against the allowed pattern list
			for _, file := range modifiedFiles {
				isAllowed := false
				for _, pattern := range analysis.AffectedFiles {
					if matchAffectedFile(pattern, file) {
						isAllowed = true
						break
					}
				}
				if !isAllowed {
					return fmt.Errorf("security violation: patch attempts to modify file %q which is not in the approved affected_files spec %v", file, analysis.AffectedFiles)
				}
			}
		}
	}

	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)

	if task.RepositoryID != nil {
		// Single repo
		targetPath := o.hostWorktreePath(task, localPath, worktreeSuffix)
		fullPath := filepath.Join(targetPath, "patch.diff")
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(patchText), 0o644); err != nil {
			return err
		}

		containerTargetPath := o.containerPathForHostPath(task, targetPath, worktreeSuffix)
		containerPatchPath := filepath.Join(containerTargetPath, "patch.diff")

		cmd := fmt.Sprintf("git -C %[1]s apply --recount --whitespace=nowarn %[2]s || patch -d %[1]s -p1 < %[2]s || patch -d %[1]s -p0 < %[2]s",
			quoteShellArg(containerTargetPath),
			quoteShellArg(containerPatchPath),
		)
		_, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_apply_patch", cmd, worktreeSuffix)
		if err != nil {
			return fmt.Errorf("git apply patch: %w", err)
		}
		_, _ = o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch", fmt.Sprintf("rm %s", quoteShellArg(containerPatchPath)), worktreeSuffix)
		return nil
	}

	// Multi-repo: split patch by repository
	repoPatches := splitPatchByRepo(patchText)
	for repoName, repoPatchText := range repoPatches {
		repoHostPath := filepath.Join(localPath, repoName)
		repoWorktreeHostPath := o.hostWorktreePath(task, repoHostPath, worktreeSuffix)
		
		fullPath := filepath.Join(repoWorktreeHostPath, "patch.diff")
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(repoPatchText), 0o644); err != nil {
			return err
		}

		containerRepoWorktreePath := o.containerPathForHostPath(task, repoWorktreeHostPath, worktreeSuffix)
		containerPatchPath := filepath.Join(containerRepoWorktreePath, "patch.diff")

		// Use -p2 because splitPatchByRepo keeps a/repoName/ prefix, which needs to be stripped relative to repoWorktree root.
		cmd := fmt.Sprintf("git -C %[1]s apply -p2 --recount --whitespace=nowarn %[2]s || patch -d %[1]s -p2 < %[2]s",
			quoteShellArg(containerRepoWorktreePath),
			quoteShellArg(containerPatchPath),
		)
		_, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_apply_patch_"+repoName, cmd, worktreeSuffix)
		if err != nil {
			return fmt.Errorf("git apply patch failed for repo %s: %w", repoName, err)
		}
		_, _ = o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_clean_patch_"+repoName, fmt.Sprintf("rm %s", quoteShellArg(containerPatchPath)), worktreeSuffix)
	}
	return nil
}

func (o *Orchestrator) captureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, worktreeSuffix string) (string, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	targetPath := o.hostWorktreePath(task, localPath, worktreeSuffix)
	containerTargetPath := o.containerPathForHostPath(task, targetPath, worktreeSuffix)

	if task.RepositoryID != nil {
		out, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_git_diff", fmt.Sprintf("git -C %s diff", quoteShellArg(containerTargetPath)), worktreeSuffix)
		if err != nil {
			return "", fmt.Errorf("git diff failed: %w", err)
		}
		diffText, _ := out["stdout"].(string)
		return diffText, nil
	}

	// Multi-repo diff
	var pattern string
	if worktreeSuffix != "" {
		pattern = fmt.Sprintf("*%s", worktreeSuffix)
	} else {
		pattern = "*"
	}
	out, err := o.runSandboxStepInWorktree(ctx, task, agent, stepID+"_git_diff_multi", fmt.Sprintf(`
		DIFF_OUT=""
		for d in %[1]s/%[2]s/ ; do
			if [ -d "$d/.git" ]; then
				pushd "$d" > /dev/null
				REPO_DIFF=$(git diff)
				if [ -n "$REPO_DIFF" ]; then
					d_name=$(basename "$d")
					repo_display="${d_name%%%[3]s}"
					DIFF_OUT="${DIFF_OUT}--- Repository: ${repo_display}\n${REPO_DIFF}\n\n"
				fi
				popd > /dev/null
			fi
		done
		echo -e "$DIFF_OUT"
	`, quoteShellArg(containerTargetPath), pattern, worktreeSuffix), worktreeSuffix)
	if err != nil {
		return "", fmt.Errorf("multi-repo git diff failed: %w", err)
	}
	diffText, _ := out["stdout"].(string)
	return diffText, nil
}

func (o *Orchestrator) saveArtifact(ctx context.Context, jobID string, taskID string, step string, artType string, payload any) error {
	if o.artifacts == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	artifact := &models.WorkflowArtifact{
		JobID:   jobID,
		TaskID:  taskID,
		Step:    step,
		Type:    artType,
		Payload: raw,
	}
	return o.artifacts.Create(ctx, artifact)
}

func extractPatch(parsed map[string]any) string {
	if parsed == nil {
		return ""
	}
	var p string
	if v, ok := parsed["patch"].(string); ok && v != "" {
		p = v
	} else if v, ok := parsed["patch_text"].(string); ok && v != "" {
		p = v
	} else if v, ok := parsed["diff"].(string); ok && v != "" {
		p = v
	}
	if p == "" {
		return ""
	}
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "```") {
		lines := strings.Split(p, "\n")
		if len(lines) >= 2 {
			endIdx := len(lines) - 1
			for i := len(lines) - 1; i > 0; i-- {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					endIdx = i
					break
				}
			}
			p = strings.Join(lines[1:endIdx], "\n")
		}
	}
	return strings.TrimSpace(p) + "\n"
}

func deriveChangeName(task *models.Task) string {
	slug := strings.ToLower(task.Title)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "task-" + task.ID
		if len(slug) > 13 {
			slug = slug[:13]
		}
	}
	return slug
}

func taskReadyForExecution(task *models.Task) bool {
	switch task.SpecStatus {
	case models.TaskSpecStatusApproved, models.TaskSpecStatusAutoApproved:
		return true
	default:
		return false
	}
}

func matchAffectedFile(pattern, file string) bool {
	pattern = strings.TrimSpace(pattern)
	file = strings.TrimSpace(file)
	if pattern == "" || file == "" {
		return false
	}

	// 1. Exact or Clean match
	if pattern == file || filepath.Clean(pattern) == filepath.Clean(file) {
		return true
	}

	// 2. Glob match
	if strings.ContainsAny(pattern, "*?[]") {
		if matched, err := filepath.Match(pattern, file); err == nil && matched {
			return true
		}
	}

	// If the pattern has no spaces, it is considered a specific filename or path,
	// so we don't apply loose description-based heuristics below.
	if !strings.Contains(pattern, " ") {
		return false
	}

	lowerPattern := strings.ToLower(pattern)

	// 3. Documentation file heuristic
	if strings.Contains(lowerPattern, "documentation") {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".md" || ext == ".txt" || ext == ".rst" || strings.Contains(strings.ToLower(file), "/docs/") || strings.HasPrefix(strings.ToLower(file), "docs/") {
			return true
		}
	}

	// 4. Script file heuristic
	if strings.Contains(lowerPattern, "script") {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".sh" || ext == ".py" || ext == ".bash" || ext == ".bat" || strings.Contains(strings.ToLower(file), "/scripts/") || strings.HasPrefix(strings.ToLower(file), "scripts/") {
			return true
		}
	}

	// 5. Extract and check extensions (e.g. "(.go)", "GoLang source files (.go)")
	extRegex := regexp.MustCompile(`\.([a-zA-Z0-9]+)`)
	exts := extRegex.FindAllString(pattern, -1)
	fileExt := filepath.Ext(file)
	for _, ext := range exts {
		cleanExt := strings.TrimRight(ext, " )]}.,;")
		if strings.EqualFold(cleanExt, fileExt) {
			return true
		}
	}

	// 6. Catch-all descriptive patterns
	if strings.Contains(lowerPattern, "all relevant source files") ||
		strings.Contains(lowerPattern, "all source files") ||
		strings.Contains(lowerPattern, "any file") ||
		strings.Contains(lowerPattern, "project files") {
		return true
	}

	// 7. Check if the filename itself is explicitly mentioned in the description
	baseName := filepath.Base(file)
	if strings.Contains(lowerPattern, strings.ToLower(baseName)) {
		return true
	}

	return false
}

func (o *Orchestrator) runTargetedTests(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepName string, changedFiles []string, worktreeSuffix string) (map[string]any, error) {
	if len(changedFiles) == 0 {
		return map[string]any{"status": "skipped", "reason": "no changed files"}, nil
	}

	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	
	// Determine the host directory that is mounted as /workspace
	mountedHostPath := localPath
	if worktreeSuffix != "" && task.RepositoryID != nil {
		mountedHostPath = localPath + worktreeSuffix
	}

	// Group changed files by their nearest module directory (relative to mountedHostPath)
	type moduleGroup struct {
		isGo         bool
		isJS         bool
		files        []string
		goPackages   map[string]bool
	}
	
	groups := make(map[string]*moduleGroup)

	for _, file := range changedFiles {
		ext := filepath.Ext(file)
		var markers []string
		isGo := false
		isJS := false
		if ext == ".go" {
			markers = []string{"go.mod"}
			isGo = true
		} else if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
			markers = []string{"package.json"}
			isJS = true
		} else {
			if _, err := os.Stat(filepath.Join(mountedHostPath, filepath.Dir(file), "go.mod")); err == nil {
				markers = []string{"go.mod"}
				isGo = true
			} else if _, err := os.Stat(filepath.Join(mountedHostPath, filepath.Dir(file), "package.json")); err == nil {
				markers = []string{"package.json"}
				isJS = true
			}
		}

		modRelDir := ""
		relFile := file
		if len(markers) > 0 {
			if dir, rf, found := findModuleDir(mountedHostPath, file, markers); found {
				modRelDir = dir
				relFile = rf
			}
		}

		g, ok := groups[modRelDir]
		if !ok {
			g = &moduleGroup{
				goPackages: make(map[string]bool),
			}
			groups[modRelDir] = g
		}
		if isGo {
			g.isGo = true
			dir := filepath.Dir(relFile)
			if dir == "." {
				g.goPackages["."] = true
			} else {
				g.goPackages["./"+dir] = true
			}
		} else if isJS {
			g.isJS = true
			g.files = append(g.files, relFile)
		}
	}

	var testErrors []string
	var testResults []map[string]any

	for modRelDir, g := range groups {
		var cmd string
		var detectedType string
		
		containerModPath := "/workspace"
		if modRelDir != "" {
			containerModPath = filepath.Join("/workspace", modRelDir)
		}

		if g.isGo {
			detectedType = "go"
			pkgs := []string{}
			for pkg := range g.goPackages {
				pkgs = append(pkgs, pkg+"/...")
			}
			cmd = fmt.Sprintf("cd %s && go test -v %s", quoteShellArg(containerModPath), strings.Join(pkgs, " "))
		} else if g.isJS {
			detectedType = "javascript"
			var quotedFiles []string
			for _, f := range g.files {
				quotedFiles = append(quotedFiles, quoteShellArg(f))
			}
			cmd = fmt.Sprintf("cd %s && (npm test -- --findRelatedTests %s || npm test -- %s || npm test)", quoteShellArg(containerModPath), strings.Join(quotedFiles, " "), strings.Join(quotedFiles, " "))
		} else {
			absModPath := filepath.Join(mountedHostPath, modRelDir)
			if _, err := os.Stat(filepath.Join(absModPath, "go.mod")); err == nil {
				cmd = fmt.Sprintf("cd %s && go test ./...", quoteShellArg(containerModPath))
				detectedType = "go"
			} else if _, err := os.Stat(filepath.Join(absModPath, "package.json")); err == nil {
				cmd = fmt.Sprintf("cd %s && npm test", quoteShellArg(containerModPath))
				detectedType = "javascript"
			} else {
				continue
			}
		}

		o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("running targeted tests for %s in %s: %s", detectedType, containerModPath, cmd))

		out, err := o.runSandboxStepInWorktree(ctx, task, agent, stepName, cmd, worktreeSuffix)
		if err != nil {
			o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("targeted tests execution failed in %s: %v", containerModPath, err))
			_ = o.saveArtifact(ctx, jobID, task.ID, stepName, "targeted_test", map[string]any{
				"status":  "failed",
				"error":   err.Error(),
				"command": cmd,
				"type":    detectedType,
				"module":  modRelDir,
			})
			testErrors = append(testErrors, fmt.Sprintf("module %s: %v", modRelDir, err))
		} else {
			stdout, _ := out["stdout"].(string)
			result := map[string]any{
				"status":  "passed",
				"stdout":  stdout,
				"command": cmd,
				"type":    detectedType,
				"module":  modRelDir,
			}
			_ = o.saveArtifact(ctx, jobID, task.ID, stepName, "targeted_test", result)
			testResults = append(testResults, result)
		}
	}

	if len(testErrors) > 0 {
		return nil, fmt.Errorf("targeted tests failed: %s", strings.Join(testErrors, "; "))
	}

	if len(testResults) == 0 {
		return map[string]any{"status": "skipped", "reason": "no tests ran"}, nil
	}

	return map[string]any{
		"status": "passed",
		"info":   fmt.Sprintf("%d test suites passed", len(testResults)),
	}, nil
}

func (o *Orchestrator) runSandboxStepInWorktree(ctx context.Context, task *models.Task, agent *models.Agent, stepID, command string, worktreeSuffix string) (map[string]any, error) {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	hostWorkspacePath := localPath
	if worktreeSuffix != "" && task.RepositoryID != nil {
		hostWorkspacePath = localPath + worktreeSuffix
	}

	ctx, span := otel.Tracer("auto-code-os/orchestrator").Start(ctx, "orchestrator.sandbox_step")
	defer span.End()
	result, err := o.runtime.Run(ctx, sandbox.CommandRequest{
		TaskID:      task.ID,
		AgentID:     agent.ID,
		Workspace:   hostWorkspacePath,
		Command:     []string{"bash", "-lc", command},
		NetworkMode: sandbox.NetworkModeNone,
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Stdout) != "" {
		o.log(ctx, task.ID, nil, "info", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stdout)))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("%s: %s", stepID, strings.TrimSpace(result.Stderr)))
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s failed with exit code %d", stepID, result.ExitCode)
	}
	return map[string]any{"status": "ok", "stdout": result.Stdout}, nil
}

func (o *Orchestrator) containerPathForHostPath(task *models.Task, hostPath string, worktreeSuffix string) string {
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	activeWorkspaceHostPath := localPath
	
	if worktreeSuffix != "" {
		if task.RepositoryID != nil {
			activeWorkspaceHostPath = o.hostWorktreePath(task, localPath, worktreeSuffix)
		} else {
			activeWorkspaceHostPath = localPath
		}
	}

	rel, err := filepath.Rel(activeWorkspaceHostPath, hostPath)
	if err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return "/workspace"
		}
		return filepath.Join("/workspace", rel)
	}
	
	relMain, errMain := filepath.Rel(localPath, hostPath)
	if errMain == nil && !strings.HasPrefix(relMain, "..") {
		if relMain == "." {
			return "/workspace"
		}
		return filepath.Join("/workspace", relMain)
	}

	return "/workspace"
}

func quoteShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func readLimitedFile(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	limit := maxBytes
	if stat.Size() < limit {
		limit = stat.Size()
	}

	buf := make([]byte, limit)
	n, errRead := file.Read(buf)
	if errRead != nil && n == 0 {
		return "", errRead
	}
	
	content := string(buf[:n])
	if stat.Size() > maxBytes {
		content += "\n[TRUNCATED: file exceeded size limit]"
	}
	return content, nil
}

func findModuleDir(targetPath string, relFilePath string, markers []string) (string, string, bool) {
	absStart := filepath.Join(targetPath, relFilePath)
	dir := filepath.Dir(absStart)
	
	for {
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker)
			if stat, err := os.Stat(markerPath); err == nil && !stat.IsDir() {
				relDir, errRel := filepath.Rel(targetPath, dir)
				if errRel == nil {
					relFile, errFile := filepath.Rel(dir, absStart)
					if errFile == nil {
						return relDir, relFile, true
					}
				}
			}
		}
		if dir == targetPath || dir == filepath.Dir(dir) {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", "", false
}

func (o *Orchestrator) getChangedFiles(ctx context.Context, task *models.Task, agent *models.Agent, targetPath string, worktreeSuffix string) ([]string, error) {
	var repos []models.Repository
	var err error
	if o.repositories != nil {
		repos, err = o.repositories.ListByProjectID(ctx, task.ProjectID)
	}
	if o.repositories == nil || err != nil || len(repos) == 0 {
		containerTargetPath := o.containerPathForHostPath(task, targetPath, worktreeSuffix)
		out, err := o.runSandboxStepInWorktree(ctx, task, agent, "git_diff_names", fmt.Sprintf("git -C %s diff --name-only", quoteShellArg(containerTargetPath)), worktreeSuffix)
		if err != nil {
			return nil, err
		}
		stdout, _ := out["stdout"].(string)
		if stdout == "" {
			return nil, nil
		}
		return strings.Split(strings.TrimSpace(stdout), "\n"), nil
	}

	var targetRepos []models.Repository
	if task.RepositoryID != nil {
		for _, r := range repos {
			if r.ID == *task.RepositoryID {
				targetRepos = append(targetRepos, r)
				break
			}
		}
	} else {
		targetRepos = repos
	}

	var allChanged []string
	for _, repo := range targetRepos {
		localRepoPath := targetPath
		prefix := ""
		if task.RepositoryID == nil {
			parts := strings.Split(repo.URL, "/")
			repoName := parts[len(parts)-1]
			repoName = strings.TrimSuffix(repoName, ".git")
			localRepoPath = filepath.Join(targetPath, repoName)
			prefix = repoName + "/"
		}

		containerRepoPath := o.containerPathForHostPath(task, localRepoPath, worktreeSuffix)
		out, err := o.runSandboxStepInWorktree(ctx, task, agent, "git_diff_names_"+repo.ID, fmt.Sprintf("git -C %s diff --name-only", quoteShellArg(containerRepoPath)), worktreeSuffix)
		if err == nil {
			if stdout, ok := out["stdout"].(string); ok && stdout != "" {
				lines := strings.Split(strings.TrimSpace(stdout), "\n")
				for _, line := range lines {
					if line != "" {
						allChanged = append(allChanged, prefix+line)
					}
				}
			}
		}
	}
	return allChanged, nil
}

func (o *Orchestrator) hostWorktreePath(task *models.Task, repoPath string, worktreeSuffix string) string {
	if worktreeSuffix == "" {
		return repoPath
	}
	localPath := sandbox.WorkspacePath(o.workspaceRoot, task.ID)
	if task.RepositoryID != nil {
		clean := strings.TrimPrefix(worktreeSuffix, "-")
		clean = strings.TrimSuffix(clean, "-worktree")
		return filepath.Join(localPath, clean)
	}
	// Multi-repo
	if repoPath == localPath {
		return localPath
	}
	return repoPath + worktreeSuffix
}

func splitPatchByRepo(patchText string) map[string]string {
	repos := make(map[string]string)
	parts := strings.Split(patchText, "diff --git ")
	header := parts[0]
	
	repoBlocks := make(map[string][]string)
	for i := 1; i < len(parts); i++ {
		block := parts[i]
		if strings.HasPrefix(block, "a/") {
			sub := block[2:]
			idx := strings.Index(sub, "/")
			if idx != -1 {
				repoName := sub[:idx]
				repoBlocks[repoName] = append(repoBlocks[repoName], "diff --git "+block)
			}
		}
	}
	
	for repoName, blocks := range repoBlocks {
		repos[repoName] = header + strings.Join(blocks, "")
	}
	return repos
}


