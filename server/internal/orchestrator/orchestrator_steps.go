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

	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel"
)

func (o *Orchestrator) stepRunners(task *models.Task, agent *models.Agent, jobID string) map[string]workflow.StepFunc {
	runners := map[string]workflow.StepFunc{
		workflow.StepAnalyze: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
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
  "execution_plan": ["step-by-step", "plan", "to", "implement", "this", "task"],
  "clarification_questions": ["questions", "if", "more", "details", "are", "needed"],
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
			specStatus := models.TaskSpecStatusPendingReview
			status := models.TaskStatusSpecReview
			if len(analysis.ClarificationQuestions) > 0 {
				specStatus = models.TaskSpecStatusChangesRequested
			} else {
				autonomy := agent.AutonomyLevel
				if o.projects != nil {
					if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil && p.DefaultAutonomy != "" && autonomy == "" {
						autonomy = p.DefaultAutonomy
					}
				}
				switch autonomy {
				case models.AgentAutonomyAutonomous:
					specStatus = models.TaskSpecStatusAutoApproved
					status = models.TaskStatusCoding
				case models.AgentAutonomyApprovalRequired:
					specStatus = models.TaskSpecStatusPendingReview
					status = models.TaskStatusSpecReview
				default:
					if analysis.Complexity == models.TaskComplexityEasy {
						specStatus = models.TaskSpecStatusAutoApproved
						status = models.TaskStatusCoding
					} else {
						specStatus = models.TaskSpecStatusPendingReview
						status = models.TaskStatusSpecReview
					}
				}
			}
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
			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, backendAgent, jobID, workflow.StepCodeBackend, "Implement the backend changes. Return JSON with files_changed, summary, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, backendAgent, workflow.StepCodeBackend, patch); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, backendAgent, workflow.StepCodeBackend); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeBackend, "diff", diffText)
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
			if o.llm != nil {
				out, err := o.runLLMStep(ctx, task, frontendAgent, jobID, workflow.StepCodeFrontend, "Implement the frontend changes when applicable. Return JSON with files_changed, summary, and patch text when available.")
				if err != nil {
					return nil, err
				}
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, frontendAgent, workflow.StepCodeFrontend, patch); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, frontendAgent, workflow.StepCodeFrontend); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepCodeFrontend, "diff", diffText)
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
			diffText, err := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepMerge)
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
				diffText, _ := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepReview)
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
				if parsed, ok := out["parsed"].(map[string]any); ok {
					patch := extractPatch(parsed)
					if patch != "" {
						_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "patch", patch)
						if applyErr := o.applyPatch(ctx, task, agent, workflow.StepFix, patch); applyErr != nil {
							return nil, fmt.Errorf("apply patch: %w", applyErr)
						}
					}
				}
				if diffText, diffErr := o.captureWorkspaceDiff(ctx, task, agent, workflow.StepFix); diffErr == nil && diffText != "" {
					_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepFix, "diff", diffText)
				}
				if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); err != nil {
					return nil, err
				}
				// We don't delete review & fix checkpoints here anymore; they are skipped when resuming
				// using the job.Step filter in orchestrator_worker.go to preserve cycle counts in DB.
				return nil, workflow.ErrReviewFixLoop
			}
			return nil, fmt.Errorf("llm provider is not configured")
		},
		workflow.StepTest: func(ctx context.Context, _ workflow.StepContext) (map[string]any, error) {
			if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting); err != nil {
				return nil, err
			}
			script := `run_test() { if [ -f go.mod ]; then go test ./...; elif [ -f package.json ]; then npm test; else echo "no supported test runner found in $(pwd)"; fi; }; if [ -d .git ]; then run_test; else for d in */ ; do if [ -d "$d.git" ]; then (cd "$d" && run_test); fi; done; fi`
			out, err := o.runSandboxStep(ctx, task, agent, workflow.StepTest, script)
			if err != nil {
				return nil, err
			}
			out["exit_code"] = 0
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
			_, _ = o.captureWorkspaceDiff(ctx, task, agent, workflow.StepPR)

			// Get test results from previous step if available
			var testOut map[string]any
			if stepCtx.Inputs != nil {
				testOut = stepCtx.Inputs[workflow.StepTest]
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
				out, errSandbox := o.runSandboxStep(ctx, task, agent, "git_status_"+repo.ID, fmt.Sprintf("cd %s && git status --porcelain", localPath))
				if errSandbox != nil {
					continue // directory might not exist or not a git repo
				}
				statusOut, _ := out["stdout"].(string)
				if strings.TrimSpace(statusOut) == "" {
					continue // no changes in this repo
				}

				branchName := fmt.Sprintf("autocode/task-%s", task.ID)
				if err := o.gitOps.CreateBranch(ctx, localPath, repo.URL, branchName); err != nil {
					o.log(ctx, task.ID, nil, "warn", fmt.Sprintf("create branch failed for %s: %v", repo.URL, err))
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
				diffOut, _ := o.runSandboxStep(ctx, task, agent, "git_diff_"+repo.ID, fmt.Sprintf("cd %s && git diff main...HEAD", localPath))
				repoDiffText, _ := diffOut["stdout"].(string)
				if repoDiffText == "" {
					// Fallback to git diff
					diffOut, _ = o.runSandboxStep(ctx, task, agent, "git_diff_fallback_"+repo.ID, fmt.Sprintf("cd %s && git diff", localPath))
					repoDiffText, _ = diffOut["stdout"].(string)
				}

				prGen := NewPRGenerator()
				summary := prGen.GenerateSummary(ctx, task, agent, repoChangedFiles, repoDiffText, testOut)

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

			status := models.TaskStatusHumanReview
			if len(createdPRs) == 0 {
				status = models.TaskStatusMerged
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
						if applyErr := o.applyPatch(ctx, task, agent, stepID, patch); applyErr != nil {
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

func (o *Orchestrator) applyPatch(ctx context.Context, task *models.Task, agent *models.Agent, stepID string, patchText string) error {
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
	fullPath := filepath.Join(localPath, "patch.diff")
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(fullPath, []byte(patchText), 0o644); err != nil {
		return err
	}
	_, err := o.runSandboxStep(ctx, task, agent, stepID+"_apply_patch", "git apply --recount --whitespace=nowarn patch.diff || patch -p1 < patch.diff || patch -p0 < patch.diff")
	if err != nil {
		return fmt.Errorf("git apply patch: %w", err)
	}
	_, _ = o.runSandboxStep(ctx, task, agent, stepID+"_clean_patch", "rm patch.diff")
	return nil
}

func (o *Orchestrator) captureWorkspaceDiff(ctx context.Context, task *models.Task, agent *models.Agent, stepID string) (string, error) {
	if task.RepositoryID != nil {
		out, err := o.runSandboxStep(ctx, task, agent, stepID+"_git_diff", "git diff")
		if err != nil {
			return "", fmt.Errorf("git diff failed: %w", err)
		}
		diffText, _ := out["stdout"].(string)
		return diffText, nil
	}

	// Multi-repo diff
	out, err := o.runSandboxStep(ctx, task, agent, stepID+"_git_diff_multi", `
		DIFF_OUT=""
		for d in */ ; do
			if [ -d "$d/.git" ]; then
				pushd "$d" > /dev/null
				REPO_DIFF=$(git diff)
				if [ -n "$REPO_DIFF" ]; then
					DIFF_OUT="${DIFF_OUT}--- Repository: ${d%/}\n${REPO_DIFF}\n\n"
				fi
				popd > /dev/null
			fi
		done
		echo -e "$DIFF_OUT"
	`)
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
	if p, ok := parsed["patch"].(string); ok && p != "" {
		return p
	}
	if p, ok := parsed["patch_text"].(string); ok && p != "" {
		return p
	}
	if p, ok := parsed["diff"].(string); ok && p != "" {
		return p
	}
	return ""
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
