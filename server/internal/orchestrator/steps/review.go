package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/llm"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func hasActionableFindings(findings []models.ReviewFinding) bool {
	for _, f := range findings {
		if f.RequiresFix {
			return true
		}
		s := strings.ToLower(strings.TrimSpace(f.Severity))
		if s == "warning" || s == "error" || s == "high" || s == "blocking" || s == "critical" || s == "medium" {
			return true
		}
	}
	return false
}

func ParseReviewFindings(parsed map[string]any) ([]models.ReviewFinding, error) {
	if parsed == nil {
		return nil, nil
	}

	var rawFindings []any

	if findings, exists := parsed["findings"]; exists {
		if slice, ok := findings.([]any); ok {
			rawFindings = slice
		} else if item, ok := findings.(map[string]any); ok {
			rawFindings = []any{item}
		}
	} else if arr, exists := parsed["array"]; exists {
		if slice, ok := arr.([]any); ok {
			rawFindings = slice
		} else if item, ok := arr.(map[string]any); ok {
			rawFindings = []any{item}
		}
	} else if _, hasFile := parsed["file"]; hasFile {
		rawFindings = []any{parsed}
	} else if _, hasRec := parsed["recommendation"]; hasRec {
		rawFindings = []any{parsed}
	}

	var result []models.ReviewFinding
	for _, raw := range rawFindings {
		fMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var f models.ReviewFinding
		if repo, ok := fMap["repo"].(string); ok {
			f.Repo = repo
		}
		if file, ok := fMap["file"].(string); ok {
			f.File = file
		}
		if lineVal, ok := fMap["line"]; ok {
			switch val := lineVal.(type) {
			case float64:
				f.Line = int(val)
			case int:
				f.Line = val
			}
		}
		if severity, ok := fMap["severity"].(string); ok {
			f.Severity = severity
		}
		if reqFix, ok := fMap["requires_fix"].(bool); ok {
			f.RequiresFix = reqFix
		}
		if recommendation, ok := fMap["recommendation"].(string); ok {
			f.Recommendation = recommendation
		} else if rec, ok := fMap["recommendation_text"].(string); ok {
			f.Recommendation = rec
		} else if rec, ok := fMap["message"].(string); ok {
			f.Recommendation = rec
		}
		result = append(result, f)
	}

	return result, nil
}

func getCoderModel(ctx context.Context, lister CheckpointLister, taskID string) string {
	if lister == nil {
		return ""
	}
	checkpoints, err := lister.ListCheckpoints(ctx, taskID)
	if err != nil || len(checkpoints) == 0 {
		return ""
	}
	// Search backwards for the most recent coding or fix step
	for i := len(checkpoints) - 1; i >= 0; i-- {
		cp := checkpoints[i]
		if cp.Step == workflow.StepCodeBackend || cp.Step == workflow.StepCodeFrontend || cp.Step == workflow.StepFix {
			var state map[string]any
			if err := json.Unmarshal(cp.State, &state); err == nil {
				if model, ok := state["model"].(string); ok && model != "" {
					return model
				}
			}
		}
	}
	return ""
}

// ReviewStep implements Step for the automated review phase.
type ReviewStep struct {
	rt               StepRuntime
	tasks            TaskReader
	projects         ProjectReader
	llm              LLMRunner
	diff             DiffCapturer
	artifacts        ArtifactSaver
	assigner         ReviewerAssigner
	checkpoints      CheckpointReader
	checkpointLister CheckpointLister
	status           StatusUpdater
	log              Logger
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
	checkpointLister CheckpointLister,
	status StatusUpdater,
	log Logger,
) *ReviewStep {
	return &ReviewStep{
		rt:               rt,
		tasks:            tasks,
		projects:         projects,
		llm:              llm,
		diff:             diff,
		artifacts:        artifacts,
		assigner:         assigner,
		checkpoints:      checkpoints,
		checkpointLister: checkpointLister,
		status:           status,
		log:              log,
	}
}

func (s *ReviewStep) ID() string { return workflow.StepReview }

func (s *ReviewStep) StatusOnResume(output StepResult) string {
	if limitReached, ok := output["cycle_limit_reached"].(bool); ok && limitReached {
		return models.TaskStatusTesting
	}
	nextStatus := models.TaskStatusTesting
	if parsed, ok := output["parsed"].(map[string]any); ok {
		findings, _ := ParseReviewFindings(parsed)
		if len(findings) > 0 && hasActionableFindings(findings) {
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
		rev, err := s.assigner.AssignReviewer(ctx, s.rt.Task)
		if rev != nil {
			reviewerAgent = rev
			assignedAgentID = rev.ID
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("assigned reviewer agent %s for review step", reviewerAgent.Name))
			}
		}
		if err != nil {
			if assignedAgentID != "" {
				if releaser, ok := s.assigner.(AgentReleaser); ok {
					_ = releaser.Release(context.WithoutCancel(ctx), assignedAgentID)
				}
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
		var analysis models.TaskAnalysis
		if len(s.rt.Task.Analysis) > 0 {
			_ = json.Unmarshal(s.rt.Task.Analysis, &analysis)
		}
		frozen := LoadFrozenContext(stepCtx, &analysis)

		if frozen != nil {
			if frozen.TasksMD != "" {
				instruction += "\n\nCRITICAL RUBRIC - You MUST verify that the following tasks have been completed correctly and accurately based on the diff. If any task is incomplete or not addressed, report a finding with 'requires_fix': true:\n\n" + frozen.TasksMD
			}
			if len(frozen.AcceptanceCriteria) > 0 {
				acJSON, _ := json.MarshalIndent(frozen.AcceptanceCriteria, "", "  ")
				instruction += "\n\nACCEPTANCE CRITERIA - You MUST verify the code meets these criteria:\n```json\n" + string(acJSON) + "\n```"
			}
			if len(frozen.ExecutionBoundaries) > 0 {
				ebJSON, _ := json.MarshalIndent(frozen.ExecutionBoundaries, "", "  ")
				instruction += "\n\nEXECUTION BOUNDARIES - You MUST verify the code modifications did not violate these boundaries:\n```json\n" + string(ebJSON) + "\n```"
			}
		}

		// Harness Independence: Exclude the model used in the preceding coding step
		var routeTrace *llm.RouteTrace
		coderModel := getCoderModel(ctx, s.checkpointLister, s.rt.Task.ID)
		if coderModel != "" {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", fmt.Sprintf("harness independence: excluding coder model %s for review", coderModel))
			}
			ctx = llm.WithExcludeModelID(ctx, coderModel)
			ctx, routeTrace = llm.WithRouteTrace(ctx)
		}

		out, err := s.llm.RunLLMStep(ctx, s.rt.Task, reviewerAgent, s.rt.JobID, workflow.StepReview, instruction)
		if err != nil {
			return nil, err
		}
		if routeTrace != nil && routeTrace.SelfReviewFallback {
			if s.log != nil {
				s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("harness independence fallback: reviewed by the same model that wrote the code (%s) — no alternative model was available", routeTrace.ActualModel))
			}
			out["self_review_fallback"] = true
			out["self_review_fallback_model"] = routeTrace.ActualModel
		}
		hasFindings := false
		if parsed, ok := out["parsed"].(map[string]any); ok {
			if s.artifacts != nil {
				_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepReview, "review_findings", parsed)
			}
			findings, _ := ParseReviewFindings(parsed)
			if len(findings) > 0 {
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
