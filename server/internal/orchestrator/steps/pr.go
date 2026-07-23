package steps

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
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
	attestations  AttestationSigner
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
	attestations AttestationSigner,
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
		attestations:  attestations,
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

		baseBranch := repo.Branch
		if workspace != nil {
			for _, rWS := range workspace.Repos {
				if rWS.RepoID == repo.ID && rWS.DefaultBranch != "" {
					baseBranch = rWS.DefaultBranch
					break
				}
			}
		}
		if baseBranch == "" {
			baseBranch = "main"
		}

		branchName := paths.DeriveBranchName(s.rt.Task.ID, s.rt.Task.Title)
		if err := s.git.CheckoutNewBranch(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, branchName); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "error", fmt.Sprintf("checkout branch failed for %s: %v", repo.URL, err))
			return nil, fmt.Errorf("checkout branch failed for %s: %w", repo.URL, err)
		}

		// Squash all checkpoint commits into a single commit by doing a soft reset to the base branch.
		// This keeps all the changes staged.
		if err := s.git.ResetSoft(ctx, s.rt.Task, s.rt.Agent, containerLocalPath, baseBranch); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("reset soft failed for %s (squash may fail): %v", repo.URL, err))
		}

		commitMsg := fmt.Sprintf("AutoCodeOS: implement task %s\n\nTitle: %s", s.rt.Task.ID, s.rt.Task.Title)
		if err := s.gitops.CommitAndPush(ctx, localPath, repo.URL, branchName, commitMsg, nil, s.rt.Agent.Role); err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "error", fmt.Sprintf("commit and push failed for %s: %v", repo.URL, err))
			return nil, fmt.Errorf("commit and push failed for %s: %w", repo.URL, err)
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
		selfReviewFallback := reviewSelfReviewFallback(checkpoints)
		codedBy, reviewedBy := codedByReviewedBy(checkpoints)

		// Extract risk domains from task analysis
		riskDomains := extractRiskDomains(s.rt.Task)

		prGen := gitops.NewPRGenerator()
		summary := prGen.GenerateSummary(ctx, s.rt.Task, s.rt.Agent, repoChangedFiles, repoDiffText, testOutCopy, riskDomains, reviewLimitExceeded, selfReviewFallback, codedBy, reviewedBy)

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

		// Attestation (P4.3, REQ-001): sign the squashed commit that was just
		// pushed. Fail-soft — a signing/persistence error is logged and never
		// blocks PR delivery.
		if s.attestations != nil {
			commitHash, hashErr := s.git.GetHeadCommitHash(ctx, s.rt.Task, s.rt.Agent, containerLocalPath)
			if hashErr != nil || strings.TrimSpace(commitHash) == "" {
				s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("attestation skipped for %s: could not resolve HEAD commit hash: %v", repo.URL, hashErr))
			} else {
				signIn := structuredCodedByReviewedBy(checkpoints)
				signIn.RepoName = repo.URL
				signIn.CommitHash = commitHash
				signIn.TaskID = s.rt.Task.ID
				signIn.JobID = s.rt.JobID
				signIn.PromptHash = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(repoDiffText)))
				signIn.FixCyclesUsed = rejectionCount
				if s.projects != nil {
					if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
						signIn.Autonomy = p.DefaultAutonomy
						signIn.ReviewHarness = p.ReviewHarnessPolicy
					}
				}
				if err := s.attestations.SignCommit(ctx, signIn); err != nil {
					s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("attestation failed for %s commit %s: %v", repo.URL, commitHash, err))
				}
			}
		}
	}

	if s.diff != nil {
		prDiffText, err := s.diff.CapturePRDiff(ctx, s.rt.Task, s.rt.Agent, "main")
		if err != nil {
			s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to capture PR diff: %v", err))
		} else if prDiffText != "" && s.artifacts != nil {
			if err := saveArtifactWithCycleDedup(ctx, s.artifacts, s.rt.JobID, s.rt.Task.ID, stepCtx.StepID, "diff", prDiffText); err != nil {
				s.log.Log(ctx, s.rt.Task.ID, nil, "warn", fmt.Sprintf("failed to save PR diff artifact: %v", err))
			} else {
				s.log.Log(ctx, s.rt.Task.ID, nil, "info", "successfully saved PR diff artifact")
			}
		}
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
	if len(createdPRSummaries) == 0 {
		if len(s.rt.Task.PRURLs) == 0 {
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

	return StepResult{
		"status":   "pr_created",
		"branches": createdBranches,
		"pr_urls":  createdPRs,
	}, workflow.ErrWaitingApproval
}

// reviewSelfReviewFallback scans checkpoints for the most recent successful
// review step and reports whether Harness Independence had to gracefully
// fall back to reviewing with the same model that wrote the code (see
// steps/review.go's RouteTrace handling).
func reviewSelfReviewFallback(checkpoints []models.WorkflowCheckpoint) bool {
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step != workflow.StepReview {
			continue
		}
		var state map[string]any
		if json.Unmarshal(cp.State, &state) != nil {
			continue
		}
		output, ok := state["output"].(map[string]any)
		if !ok {
			continue
		}
		if flagged, _ := output["self_review_fallback"].(bool); flagged {
			return true
		}
	}
	return false
}

// structuredCodedByReviewedBy is codedByReviewedBy's structured counterpart
// for attestation building (P4.3): it returns the raw engine/provider/model
// fields instead of pre-formatted display strings.
func structuredCodedByReviewedBy(checkpoints []models.WorkflowCheckpoint) (in AttestationSignInput) {
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step != workflow.StepReview {
			continue
		}
		var state map[string]any
		if json.Unmarshal(cp.State, &state) != nil {
			continue
		}
		output, ok := state["output"].(map[string]any)
		if !ok {
			continue
		}
		if cb, ok := output["coded_by"].(map[string]any); ok {
			in.CodedByEngine, _ = cb["engine"].(string)
			in.CodedByProvider, _ = cb["provider"].(string)
			in.CodedByModel, _ = cb["model"].(string)
			if in.CodedByEngine == "" {
				in.CodedByEngine = string(models.ExecutionEngineAPINative)
			}
		}
		if rb, ok := output["reviewed_by"].(map[string]any); ok {
			provider, _ := rb["provider"].(string)
			model, _ := rb["model"].(string)
			if provider != "" || model != "" {
				in.HasReviewedBy = true
				in.ReviewedByProvider = provider
				in.ReviewedByModel = model
			}
		}
		if in.CodedByProvider != "" || in.HasReviewedBy {
			return in
		}
	}
	return in
}

// codedByReviewedBy scans checkpoints for the most recent review step's
// coded_by/reviewed_by metadata (REQ-002, cross-harness-review) and formats
// it for the PR description footer, e.g. "api_native:anthropic/claude-x" /
// "openai/gpt-x".
func codedByReviewedBy(checkpoints []models.WorkflowCheckpoint) (codedBy, reviewedBy string) {
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step != workflow.StepReview {
			continue
		}
		var state map[string]any
		if json.Unmarshal(cp.State, &state) != nil {
			continue
		}
		output, ok := state["output"].(map[string]any)
		if !ok {
			continue
		}
		if cb, ok := output["coded_by"].(map[string]any); ok {
			engine, _ := cb["engine"].(string)
			provider, _ := cb["provider"].(string)
			model, _ := cb["model"].(string)
			if provider != "" || model != "" {
				if engine == "" {
					engine = string(models.ExecutionEngineAPINative)
				}
				codedBy = fmt.Sprintf("%s:%s/%s", engine, provider, model)
			}
		}
		if rb, ok := output["reviewed_by"].(map[string]any); ok {
			provider, _ := rb["provider"].(string)
			model, _ := rb["model"].(string)
			if provider != "" || model != "" {
				reviewedBy = fmt.Sprintf("%s/%s", provider, model)
			}
		}
		if codedBy != "" || reviewedBy != "" {
			return codedBy, reviewedBy
		}
	}
	return "", ""
}

// saveArtifactWithCycleDedup saves payload as a WorkflowArtifact under step, suffixing
// "_cycle_N" when an artifact of the same step+type already exists — mirrors
// checkpoint.Store.SaveArtifact's dedup rule so repeated PR-step runs (e.g. across review/fix
// cycles) don't collide on the same artifact key. Only PRStep needs this: it holds an
// ArtifactRepository (Create + List) rather than the narrower ArtifactSaver other steps use,
// since it also reads back artifacts by job ID elsewhere in this file.
func saveArtifactWithCycleDedup(ctx context.Context, artifacts ArtifactRepository, jobID, taskID, step, artType string, payload any) error {
	dedupedStep := step
	existing, err := artifacts.ListByTaskID(ctx, taskID)
	if err == nil {
		count := 0
		for _, a := range existing {
			if (a.Step == step || strings.HasPrefix(a.Step, step+"_cycle_")) && a.Type == artType {
				count++
			}
		}
		if count > 0 {
			dedupedStep = fmt.Sprintf("%s_cycle_%d", step, count+1)
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact payload: %w", err)
	}
	return artifacts.Create(ctx, &models.WorkflowArtifact{
		JobID:   jobID,
		TaskID:  taskID,
		Step:    dedupedStep,
		Type:    artType,
		Payload: raw,
	})
}
