package orchestrator

import (
	"context"
	"fmt"
	"strings"

	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func (o *Orchestrator) executeStepTest(ctx context.Context, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	if _, err := o.updateTaskStatus(ctx, task.ID, models.TaskStatusTesting); err != nil {
		return nil, err
	}
	script := orchtester.FullVerificationScript()
	out, err := o.runSandboxStep(ctx, task, agent, workflow.StepTest, script)
	if err != nil {
		if ws, errWS := o.LoadTaskWorkspace(ctx, task); errWS == nil {
			for i := range ws.Repos {
				ws.Repos[i].Status.TestStatus = models.TestStatusFailed
			}
			_ = o.SaveTaskWorkspaceMetadata(task, ws)
		}

		// Spec 5.7 Step 9: on test failure, loop back to fix within cycle limits.
		maxCycles := 3
		if o.projects != nil {
			if p, pErr := o.projects.GetByID(ctx, task.ProjectID); pErr == nil && p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
		}
		reviewCycleCount := o.countSuccessfulCheckpoints(ctx, task.ID, workflow.StepReview)
		if reviewCycleCount < maxCycles {
			o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("tests failed, looping back to review-fix (cycle %d/%d): %v", reviewCycleCount+1, maxCycles, err))
			if _, statusErr := o.updateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); statusErr != nil {
				return nil, statusErr
			}
			return nil, workflow.ErrReviewFixLoop
		}
		o.log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("tests failed and review-fix cycle limit reached (%d/%d), failing", reviewCycleCount, maxCycles))
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
}
