package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/lib/pq"
)

// extractRiskDomains extracts risk_domains from the task's stored analysis JSON.
func extractRiskDomains(task *models.Task) []string {
	if len(task.Analysis) == 0 {
		return nil
	}
	var analysis models.TaskAnalysis
	if json.Unmarshal(task.Analysis, &analysis) == nil {
		return analysis.RiskDomains
	}
	return nil
}

func ExecutePR(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, stepCtx workflow.StepContext) (map[string]any, error) {
	if task.Status == models.TaskStatusPrReady || task.Status == models.TaskStatusHumanReview {
		return nil, workflow.ErrWaitingApproval
	}
	if deps.GitOps == nil {
		return nil, fmt.Errorf("gitops client is not configured")
	}
	targetRepos, err := deps.RepoUtil.LoadTargetRepositories(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("list project repositories: %w", err)
	}
	if len(targetRepos) == 0 {
		return nil, fmt.Errorf("no repository linked to project %s", task.ProjectID)
	}

	// Capture overall workspace diff to find changed files
	_, _ = deps.RepoUtil.CaptureWorkspaceDiff(ctx, task, agent, workflow.StepPR, "")

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

	if deps.Artifacts != nil {
		if arts, err := deps.Artifacts.ListByJobID(ctx, jobID); err == nil {
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

	var workspace *models.TaskWorkspace
	if deps.Wkspace != nil {
		ws, errWS := deps.Wkspace.LoadTaskWorkspace(ctx, task)
		if errWS == nil {
			workspace = ws
		}
	}

	for _, repo := range targetRepos {
		localPath := deps.RepoUtil.RepoHostPath(task, workspace, repo)

		// Check if this specific repo has changes before creating branch
		// For simplicity, we create branch anyway, if no changes commit will fail or we can skip.
		// A better way is to run `git status --porcelain` in localPath.
		containerLocalPath := deps.ContainerPathForHostPath(task, localPath, "")
		repoChangedFiles, errSandbox := deps.SandboxGit.GetChangedFiles(ctx, task, agent, containerLocalPath)
		if errSandbox != nil {
			continue // directory might not exist or not a git repo
		}
		if len(repoChangedFiles) == 0 && task.Complexity == models.TaskComplexityEasy {
			continue // no changes in this repo for easy tasks
		}

		branchName := fmt.Sprintf("feature/%s", task.ID)
		if err := deps.SandboxGit.CheckoutNewBranch(ctx, task, agent, containerLocalPath, branchName); err != nil {
			deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("checkout branch failed for %s: %v", repo.URL, err))
			continue
		}

		commitMsg := fmt.Sprintf("AutoCodeOS: implement task %s\n\nTitle: %s", task.ID, task.Title)
		if err := deps.GitOps.CommitAndPush(ctx, localPath, repo.URL, branchName, commitMsg, nil, agent.Role); err != nil {
			deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("commit and push failed for %s: %v", repo.URL, err))
			continue
		}

		baseBranch := repo.Branch
		if baseBranch == "" {
			baseBranch = "main"
		}

		// Capture this specific repo's diff using baseBranch, fallback to master/HEAD~1/plain diff to avoid errors
		repoDiffText, _ := deps.SandboxGit.GetPRDiff(ctx, task, agent, containerLocalPath, baseBranch)

		// Compute review limit exceeded flag
		maxReviewFixCycles := 3
		if deps.Projects != nil {
			if p, err := deps.Projects.GetByID(ctx, task.ProjectID); err == nil && p.MaxReviewFixCycles > 0 {
				maxReviewFixCycles = p.MaxReviewFixCycles
			}
		}
		checkpoints, _ := deps.Workflows.ListCheckpoints(ctx, task.ID)
		rejectionCount := 0
		for _, cp := range checkpoints {
			if cp.Step == "pr_rejection" {
				rejectionCount++
			}
		}
		reviewLimitExceeded := rejectionCount >= maxReviewFixCycles-1

		// Extract risk domains from task analysis
		riskDomains := extractRiskDomains(task)

		prGen := gitops.NewPRGenerator()
		summary := prGen.GenerateSummary(ctx, task, agent, repoChangedFiles, repoDiffText, testOutCopy, riskDomains, reviewLimitExceeded)

		prURL, err := deps.GitOps.CreatePullRequest(ctx, repo.URL, branchName, summary.Title, summary.Body)
		if err != nil {
			deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("create PR failed for %s: %v", repo.URL, err))
			continue
		}
		if strings.TrimSpace(prURL) == "" {
			deps.Log(ctx, task.ID, nil, "info", fmt.Sprintf("create PR returned no URL for %s; skipping", repo.URL))
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
		if _, err := deps.Tasks.Update(ctx, task.ID, models.UpdateTaskInput{
			PRURLs:     &pqCreatedPRs,
			PRMetadata: prMetadataRaw,
		}); err != nil {
			deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save PR metadata: %v", err))
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
		if _, err := deps.Tasks.Update(ctx, task.ID, models.UpdateTaskInput{
			PRMetadata: noChangesMetadata,
		}); err != nil {
			deps.Log(ctx, task.ID, nil, "warn", fmt.Sprintf("failed to save PR metadata for no-changes: %v", err))
		}
	}
	if _, err := deps.UpdateTaskStatus(ctx, task.ID, status); err != nil {
		return nil, err
	}

	if len(createdPRs) == 0 {
		return map[string]any{
			"status":   "no_changes_detected",
			"branches": createdBranches,
			"pr_urls":  createdPRs,
		}, nil
	}

	return nil, workflow.ErrWaitingApproval
}
