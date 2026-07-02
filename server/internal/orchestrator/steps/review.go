package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func hasActionableFindings(findings any) bool {
	slice, ok := findings.([]any)
	if !ok {
		return false
	}
	for _, f := range slice {
		fMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if reqFix, ok := fMap["requires_fix"].(bool); ok && reqFix {
			return true
		}
		sev, ok := fMap["severity"].(string)
		if ok {
			s := strings.ToLower(strings.TrimSpace(sev))
			if s == "warning" || s == "error" || s == "high" || s == "blocking" || s == "critical" || s == "medium" {
				return true
			}
		}
	}
	return false
}

func getReviewFindings(parsed map[string]any) any {
	if parsed == nil {
		return nil
	}
	if findings, exists := parsed["findings"]; exists {
		return findings
	}
	if arr, exists := parsed["array"]; exists {
		return arr
	}
	// Fallback if the parsed map itself represents a single finding object
	if _, hasFile := parsed["file"]; hasFile {
		return []any{parsed}
	}
	if _, hasRec := parsed["recommendation"]; hasRec {
		return []any{parsed}
	}
	return nil
}

// ReviewStep implements Step for the automated review phase.
type ReviewStep struct {
	rt          StepRuntime
	tasks       TaskReader
	projects    ProjectReader
	llm         LLMRunner
	diff        DiffCapturer
	artifacts   ArtifactSaver
	assigner    ReviewerAssigner
	checkpoints CheckpointReader
	status      StatusUpdater
	log         Logger
}

func NewReviewStep(
	rt StepRuntime,
	tasks TaskReader,
	projects ProjectReader,
	llm LLMRunner,
	diff DiffCapturer,
	artifacts ArtifactSaver,
	assigner ReviewerAssigner,
	checkpoints CheckpointReader,
	status StatusUpdater,
	log Logger,
) *ReviewStep {
	return &ReviewStep{
		rt:          rt,
		tasks:       tasks,
		projects:    projects,
		llm:         llm,
		diff:        diff,
		artifacts:   artifacts,
		assigner:    assigner,
		checkpoints: checkpoints,
		status:      status,
		log:         log,
	}
}

func (s *ReviewStep) ID() string { return workflow.StepReview }

func (s *ReviewStep) StatusOnResume(output StepResult) string {
	if limitReached, ok := output["cycle_limit_reached"].(bool); ok && limitReached {
		return models.TaskStatusTesting
	}
	nextStatus := models.TaskStatusTesting
	if parsed, ok := output["parsed"].(map[string]any); ok {
		findings := getReviewFindings(parsed)
		if findings != nil && hasActionableFindings(findings) {
			nextStatus = models.TaskStatusFixing
		}
	}
	return nextStatus
}

func (s *ReviewStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	t, err := s.tasks.GetByID(ctx, s.rt.Task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return StepResult{"status": "skipped", "info": "skipped review step for easy task"}, nil
	}
	if s.rt.Task.Status == models.TaskStatusFixing || s.rt.Task.Status == models.TaskStatusTesting {
		return StepResult{"status": "bypassed_via_human_review"}, nil
	}
	if s.rt.Task.Status == models.TaskStatusHumanReview {
		return nil, workflow.ErrWaitingApproval
	}
	reviewerAgent := s.rt.Agent
	assignedAgentID := ""
	if s.assigner != nil {
		if rev, err := s.assigner.AssignReviewer(ctx, s.rt.Task); err == nil && rev != nil {
			reviewerAgent = rev
			assignedAgentID = rev.ID
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned reviewer agent %s for review step", reviewerAgent.Name))
			}
		}
	}
	if assignedAgentID != "" && (s.rt.Agent == nil || assignedAgentID != s.rt.Agent.ID) {
		defer func() {
			if releaser, ok := s.assigner.(AgentReleaser); ok {
				if err := releaser.Release(context.WithoutCancel(ctx), assignedAgentID); err != nil {
					if s.log != nil {
						s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("release reviewer agent failed: %v", err))
					}
				}
			}
		}()
	}

	// Enforce review-fix cycle limit.
	maxCycles := 3
	if s.projects != nil {
		if p, err := s.projects.GetByID(ctx, s.rt.Task.ProjectID); err == nil {
			if p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
			if p.AutoReviewPolicy == "human_only" {
				if s.rt.Task.Status != models.TaskStatusHumanReview {
					if s.status != nil {
						_, _ = s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusHumanReview)
					}
				}
				return nil, workflow.ErrWaitingApproval
			}
		}
	}
	reviewCycleCount := 0
	if s.checkpoints != nil {
		reviewCycleCount = s.checkpoints.CountSuccessful(ctx, s.rt.Task.ID, workflow.StepReview)
	}
	if s.llm != nil {
		var diffText string
		if s.diff != nil {
			var err error
			diffText, err = s.diff.CapturePRDiff(ctx, s.rt.Task, s.rt.Agent, "main")
			if err != nil || diffText == "" {
				suffix := ""
				if s.rt.Agent != nil {
					if s.rt.Agent.Role == models.AgentRoleBackend {
						suffix = "-be-worktree"
					} else if s.rt.Agent.Role == models.AgentRoleFrontend {
						suffix = "-fe-worktree"
					}
				}
				diffText, _ = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepReview, suffix)
			}
		}
		if diffText == "" && s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "no diff was provided to review step")
		}
		instruction := "Review the proposed changes. Here is the current workspace diff:\n\n" + diffText + "\n\n" +
			"Return JSON findings with severity, file, line, and recommendation."
		if len(s.rt.Task.Analysis) > 0 {
			var analysis models.TaskAnalysis
			if err := json.Unmarshal(s.rt.Task.Analysis, &analysis); err == nil && analysis.TasksMD != "" {
				instruction += "\n\nCRITICAL RUBRIC - You MUST verify that the following tasks have been completed correctly and accurately based on the diff. If any task is incomplete or not addressed, report a finding with 'requires_fix': true:\n\n" + analysis.TasksMD
			}
		}
		out, err := s.llm.RunLLMStep(ctx, s.rt.Task, reviewerAgent, s.rt.JobID, workflow.StepReview, instruction)
		if err != nil {
			return nil, err
		}
		hasFindings := false
		if parsed, ok := out["parsed"].(map[string]any); ok {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepReview, "review_findings", parsed)
			}
			findings := getReviewFindings(parsed)
			if findings != nil {
				if hasActionableFindings(findings) {
					hasFindings = true
				}
			}
		}
		nextStatus := models.TaskStatusFixing
		if !hasFindings {
			nextStatus = models.TaskStatusTesting
		}
		// If we've exceeded the cycle limit, skip fix and proceed to test.
		if hasFindings && reviewCycleCount >= maxCycles {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("review-fix cycle limit reached (%d/%d), proceeding to test despite findings", reviewCycleCount, maxCycles))
			}
			nextStatus = models.TaskStatusTesting
			out["cycle_limit_reached"] = true
		}
		if s.status != nil {
			if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, nextStatus); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
