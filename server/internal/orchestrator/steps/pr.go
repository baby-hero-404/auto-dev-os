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

// PRStep implements Step for the pull request generation phase.
type PRStep struct {
	rt            StepRuntime
	tasks         TaskRepository
	status        StatusUpdater
	worktree      WorktreeManager
	workspace     WorkspaceLoader
	git           SandboxGitClient
	diff          DiffCapturer
	artifacts     ArtifactRepository
	projects      ProjectReader
	checkpoints   CheckpointLister
	gitops        GitOpsClient
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string
	log           Logger
}

func NewPRStep(
	rt StepRuntime,
	tasks TaskRepository,
	status StatusUpdater,
	worktree WorktreeManager,
	workspace WorkspaceLoader,
	git SandboxGitClient,
	diff DiffCapturer,
	artifacts ArtifactRepository,
	projects ProjectReader,
	checkpoints CheckpointLister,
	gitops GitOpsClient,
	containerPath func(task *models.Task, hostPath string, worktreeSuffix string) string,
	log Logger,
) *PRStep {
	return &PRStep{
		rt:            rt,
		tasks:         tasks,
		status:        status,
		worktree:      worktree,
		workspace:     workspace,
		git:           git,
		diff:          diff,
		artifacts:     artifacts,
		projects:      projects,
		checkpoints:   checkpoints,
		gitops:        gitops,
		containerPath: containerPath,
		log:           log,
	}
}

func (s *PRStep) ID() string                         { return workflow.StepPR }
func (s *PRStep) StatusOnResume(_ StepResult) string { return models.TaskStatusHumanReview }

func (s *PRStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	if s.rt.Task.Status == models.TaskStatusPrReady || s.rt.Task.Status == models.TaskStatusHumanReview {
		return nil, workflow.ErrWaitingApproval
	}
	if s.gitops == nil {
		return nil, fmt.Errorf("gitops client is not configured")
	}
	var targetRepos []models.Repository
	if s.worktree != nil {
		var err error
		targetRepos, err = s.worktree.LoadTargetRepositories(ctx, s.rt.Task)
		if err != nil {
			return nil, fmt.Errorf("list project repositories: %w", err)
		}
	}
	if len(targetRepos) == 0 {
		return nil, fmt.Errorf("no repository linked to project %s", s.rt.Task.ProjectID)
	}

	// Capture overall workspace diff to find changed files
	if s.diff != nil {
		_, _ = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepPR, "")
	}

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

	if s.artifacts != nil {
		if arts, err := s.artifacts.ListByJobID(ctx, s.rt.JobID); err == nil {
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
	if s.workspace != nil {
		ws, errWS := s.workspace.LoadTaskWorkspace(ctx, s.rt.Task)
		if errWS == nil {
			workspace = ws
		}
	}

	for _, repo := range targetRepos {
		var localPath string
		if s.worktree != nil {
			localPath = s.worktree.RepoHostPath(s.rt.Task, workspace, repo)
		}
		containerLocalPath := s.containerPath(s.rt.Task, localPath, "")
		repoChangedFiles, errSandbox := s.git.GetChangedFiles(ctx, s.rt.Task, s.rt.Agent, containerLocalPath)
		if errSandbox != nil {
			continue // directory might not exist or not a git repo
		}
		if len(repoChangedFiles) == 0 && s.rt.Task.Complexity == models.TaskComplexityEasy {
			continue // no changes in this repo for easy tasks
		}

		branchName := fmt.Sprintf("feature/%s", s.rt.Task.ID)
		if err := s.git.CheckoutNewBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, branchName); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "error", fmt.Sprintf("checkout branch failed for %s: %v", repo.URL, err))
			return nil, fmt.Errorf("checkout branch failed for %s: %w", repo.URL, err)
		}

		commitMsg := fmt.Sprintf("AutoCodeOS: implement task %s\n\nTitle: %s", s.rt.Task.ID, s.rt.Task.Title)
		if err := s.gitops.CommitAndPush(ctx, localPath, repo.URL, branchName, commitMsg, nil, s.rt.Agent.Role); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "error", fmt.Sprintf("commit and push failed for %s: %v", repo.URL, err))
			return nil, fmt.Errorf("commit and push failed for %s: %w", repo.URL, err)
		}

		baseBranch := repo.Branch
		if baseBranch == "" {
			baseBranch = "main"
		}

		// Capture this specific repo's diff using baseBranch, fallback to master/HEAD~1/plain diff to avoid errors
		repoDiffText, _ := s.git.GetPRDiff(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, baseBranch)

		// Compute review limit exceeded flag
		maxReviewFixCycles := 3
		if s.projects != nil {
			if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil && p.MaxReviewFixCycles > 0 {
				maxReviewFixCycles = p.MaxReviewFixCycles
			}
		}
		var checkpoints []models.WorkflowCheckpoint
		if s.checkpoints != nil {
			checkpoints, _ = s.checkpoints.ListCheckpoints(ctx, s.rt.Task.ID)
		}
		rejectionCount := 0
		for _, cp := range checkpoints {
			if cp.Step == "pr_rejection" {
				rejectionCount++
			}
		}
		reviewLimitExceeded := rejectionCount >= maxReviewFixCycles-1

		// Extract risk domains from task analysis
		riskDomains := extractRiskDomains(s.rt.Task)

		prGen := gitops.NewPRGenerator()
		summary := prGen.GenerateSummary(ctx, s.rt.Task, s.rt.Agent, repoChangedFiles, repoDiffText, testOutCopy, riskDomains, reviewLimitExceeded)

		prURL, err := s.gitops.CreatePullRequest(ctx, repo.URL, branchName, summary.Title, summary.Body)
		if err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "error", fmt.Sprintf("create PR failed for %s: %v", repo.URL, err))
			return nil, fmt.Errorf("create PR failed for %s: %w", repo.URL, err)
		}
		if strings.TrimSpace(prURL) == "" {
			s.log.Log(ctx, s.rt.Task.ID, nil, "info", fmt.Sprintf("create PR returned no URL for %s; proceeding with local changes", repo.URL))
		} else {
			createdPRs = append(createdPRs, prURL)
		}

		createdBranches = append(createdBranches, branchName)
		summary.PRURL = prURL
		createdPRSummaries = append(createdPRSummaries, *summary)
	}

	if len(createdPRSummaries) > 0 {
		var pqCreatedPRs *pq.StringArray
		if len(createdPRs) > 0 {
			arr := pq.StringArray(createdPRs)
			pqCreatedPRs = &arr
		}
		prMetadataRaw, _ := json.Marshal(createdPRSummaries)

		updateInput := models.UpdateTaskInput{
			PRMetadata: prMetadataRaw,
		}
		if pqCreatedPRs != nil {
			updateInput.PRURLs = pqCreatedPRs
		}

		if _, err := s.tasks.Update(ctx, s.rt.Task.ID, updateInput); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save PR metadata: %v", err))
		}
	}

	status := models.TaskStatusPrReady
	var shouldWait bool
	if len(createdPRSummaries) > 0 {
		shouldWait = true
	} else {
		if len(s.rt.Task.PRURLs) > 0 {
			status = models.TaskStatusPrReady
			shouldWait = true
		} else {
			status = models.TaskStatusPrReady
			shouldWait = true
			noChangesSummaries := []models.PRSummary{{
				Title:  "No changes detected",
				Body:   "No code modifications were required.",
				Status: "no_changes",
			}}
			noChangesMetadata, _ := json.Marshal(noChangesSummaries)
			if _, err := s.tasks.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{
				PRMetadata: noChangesMetadata,
			}); err != nil {
				s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save PR metadata for no-changes: %v", err))
			}
		}
	}

	if s.status != nil {
		if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, status); err != nil {
			return nil, err
		}
	}

	if shouldWait {
		return nil, workflow.ErrWaitingApproval
	}

	return StepResult{
		"status":   "no_changes_detected",
		"branches": createdBranches,
		"pr_urls":  createdPRs,
	}, nil
}
