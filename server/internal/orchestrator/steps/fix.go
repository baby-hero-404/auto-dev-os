package steps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// FixStep implements Step for fixing findings/feedback from PR review.
type FixStep struct {
	rt          StepRuntime
	tasks       TaskRepository
	checkpoints CheckpointLister
	llm         LLMRunner
	diff        DiffCapturer
	artifacts   ArtifactSaver
	patch       PatchApplier
	tests       TestRunner
	status      StatusUpdater
	log         Logger
	worktree    WorktreeManager
	fileReader  AffectedFileReader
}

func NewFixStep(
	rt StepRuntime,
	tasks TaskRepository,
	checkpoints CheckpointLister,
	llm LLMRunner,
	diff DiffCapturer,
	artifacts ArtifactSaver,
	patch PatchApplier,
	tests TestRunner,
	status StatusUpdater,
	log Logger,
	worktree WorktreeManager,
	fileReader AffectedFileReader,
) *FixStep {
	return &FixStep{
		rt:          rt,
		tasks:       tasks,
		checkpoints: checkpoints,
		llm:         llm,
		diff:        diff,
		artifacts:   artifacts,
		patch:       patch,
		tests:       tests,
		status:      status,
		log:         log,
		worktree:    worktree,
		fileReader:  fileReader,
	}
}

func (s *FixStep) ID() string { return workflow.StepFix }

func (s *FixStep) StatusOnResume(_ StepResult) string {
	return models.TaskStatusReviewing
}

func (s *FixStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	t, err := s.tasks.GetByID(ctx, s.rt.Task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return StepResult{"status": "skipped", "info": "skipped fix step for easy task"}, nil
	}

	var prFeedback string
	if s.checkpoints != nil {
		if checkpoints, cpErr := s.checkpoints.ListCheckpoints(ctx, s.rt.Task.ID); cpErr == nil {
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
	}

	if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
		if limitReached, _ := reviewOut["cycle_limit_reached"].(bool); limitReached {
			return StepResult{
				"status": "skipped",
				"info":   "review-fix cycle limit reached, skipping fix step",
			}, nil
		}
		if prFeedback == "" {
			if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
				findings := getReviewFindings(parsed)
				if findings != nil {
					if !hasActionableFindings(findings) {
						return StepResult{
							"status": "skipped",
							"info":   "no review findings, skipped fix step",
						}, nil
					}
				}
			}
		}
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
				diffText, _ = s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepFix, suffix)
			}
		}
		if diffText == "" && s.log != nil {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", "no diff was provided to fix step")
		}

		instruction := "Fix review findings. Here is the current workspace diff:\n\n" + diffText + "\n\n"

		var analysis models.TaskAnalysis
		if len(s.rt.Task.Analysis) > 0 {
			_ = json.Unmarshal(s.rt.Task.Analysis, &analysis)
		}
		if frozen := LoadFrozenContext(stepCtx, &analysis); frozen != nil {
			if len(frozen.AcceptanceCriteria) > 0 {
				acJSON, _ := json.MarshalIndent(frozen.AcceptanceCriteria, "", "  ")
				instruction += "\n\nACCEPTANCE CRITERIA - Your fix MUST still satisfy these criteria:\n```json\n" + string(acJSON) + "\n```\n"
			}
			if len(frozen.ExecutionBoundaries) > 0 {
				ebJSON, _ := json.MarshalIndent(frozen.ExecutionBoundaries, "", "  ")
				instruction += "\n\nEXECUTION BOUNDARIES - Your fix MUST NOT violate these boundaries:\n```json\n" + string(ebJSON) + "\n```\n"
			}
		}

		var findingsJSON string
		if reviewOut, ok := stepCtx.Inputs[workflow.StepReview]; ok {
			if parsed, ok := reviewOut["parsed"].(map[string]any); ok {
				findings := getReviewFindings(parsed)
				if findings != nil {
					if findingsBytes, err := json.MarshalIndent(findings, "", "  "); err == nil {
						findingsJSON = string(findingsBytes)
					}
				}
			}
		}

		if prFeedback != "" {
			instruction += fmt.Sprintf("Fix review findings and address the following PR rejection feedback:\n\n%s\n\n", prFeedback)
		} else if findingsJSON != "" {
			instruction += fmt.Sprintf("Address the following review findings:\n\n%s\n\n", findingsJSON)
		}
		instruction += "IMPORTANT: The diff above shows the current proposed changes. Use the available tools (e.g. search_replace, create_file) to fix ONLY the findings directly in the workspace. DO NOT recreate files that the diff already creates. All file paths are relative to your workspace root.\n"
		instruction += "Use run_tests/run_build/run_lint to verify your fix before finishing. When done, respond with JSON containing fixes_applied and summary."

		_, patchApplied, err := runPatchRetryLoop(ctx, patchRetryConfig{
			Task:           s.rt.Task,
			Agent:          s.rt.Agent,
			JobID:          s.rt.JobID,
			StepID:         workflow.StepFix,
			WorktreeSuffix: "",
			TestLabel:      "fix_test",
			Agentic:        true,
			MaxRetries:     3,
			LLM:            s.llm,
			Worktree:       s.worktree,
			Patcher:        s.patch,
			Diff:           s.diff,
			Artifacts:      s.artifacts,
			Tester:         s.tests,
			Tasks:          s.tasks,
			Log:            s.log,
		}, instruction)
		if err != nil {
			return nil, err
		}
		if s.diff != nil {
			if diffText, diffErr := s.diff.CaptureWorkspaceDiff(ctx, s.rt.Task, s.rt.Agent, workflow.StepFix, ""); diffErr == nil && diffText != "" {
				if s.artifacts != nil {
					_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepFix, "diff", diffText)
				}
			}
		}

		if patchApplied {
			if s.status != nil {
				if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusReviewing); err != nil {
					return nil, err
				}
			}
			// We don't delete review & fix checkpoints here anymore; they are skipped when resuming
			// using the job.Step filter in orchestrator_worker.go to preserve cycle counts in DB.
			return nil, workflow.ErrReviewFixLoop
		}

		return StepResult{
			"status": "success",
			"info":   "no fixes applied",
		}, nil
	}
	return nil, fmt.Errorf("llm provider is not configured")
}
