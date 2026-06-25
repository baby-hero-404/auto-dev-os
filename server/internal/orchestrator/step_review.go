package orchestrator

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepReview(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	t, err := o.tasks.GetByID(ctx, task.ID)
	if err == nil && t.Complexity == models.TaskComplexityEasy {
		return map[string]any{"status": "skipped", "info": "skipped review step for easy task"}, nil
	}
	if task.Status == models.TaskStatusFixing || task.Status == models.TaskStatusTesting {
		return map[string]any{"status": "bypassed_via_human_review"}, nil
	}
	if task.Status == models.TaskStatusHumanReview {
		return nil, workflow.ErrWaitingApproval
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
		if p, err := o.projects.GetByID(ctx, task.ProjectID); err == nil {
			if p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
			if p.AutoReviewPolicy == "human_only" {
				if task.Status != models.TaskStatusHumanReview {
					_, _ = o.updateTaskStatus(ctx, task.ID, models.TaskStatusHumanReview)
				}
				return nil, workflow.ErrWaitingApproval
			}
		}
	}
	reviewCycleCount := o.countSuccessfulCheckpoints(ctx, task.ID, workflow.StepReview)

	if o.llm != nil {
		diffText, _ := o.capturePRDiff(ctx, task, agent, "main")
		instruction := "Review the proposed changes. Here is the current workspace diff:\n\n" + diffText + "\n\nReturn JSON findings with severity, file, line, and recommendation."
		out, err := o.runLLMStep(ctx, task, reviewerAgent, jobID, workflow.StepReview, instruction)
		if err != nil {
			return nil, err
		}
		hasFindings := false
		if parsed, ok := out["parsed"].(map[string]any); ok {
			_ = o.saveArtifact(ctx, jobID, task.ID, workflow.StepReview, "review_findings", parsed)
			if findings, exists := parsed["findings"]; exists {
				if slice, ok := findings.([]any); ok && len(slice) > 0 {
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
}
