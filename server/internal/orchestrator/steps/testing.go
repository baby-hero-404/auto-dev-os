package steps

import (
	"context"
	"fmt"
	"strings"

	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func ExecuteTest(ctx context.Context, deps *Deps, task *models.Task, agent *models.Agent, jobID string, _ workflow.StepContext) (map[string]any, error) {
	if _, err := deps.UpdateTaskStatus(ctx, task.ID, models.TaskStatusTesting); err != nil {
		return nil, err
	}
	script := orchtester.FullVerificationScript()
	out, err := deps.RunSandboxStep(ctx, task, agent, workflow.StepTest, script)
	if err != nil {
		if deps.Wkspace != nil {
			if ws, errWS := deps.Wkspace.LoadTaskWorkspace(ctx, task); errWS == nil {
				for i := range ws.Repos {
					ws.Repos[i].Status.TestStatus = models.TestStatusFailed
				}
				_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
			}
		}

		// Spec 5.7 Step 9: on test failure, loop back to fix within cycle limits.
		maxCycles := 3
		if deps.Projects != nil {
			if p, pErr := deps.Projects.GetByID(ctx, task.ProjectID); pErr == nil && p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
		}
		reviewCycleCount := deps.Checkpoints.CountSuccessful(ctx, task.ID, workflow.StepReview)
		if reviewCycleCount < maxCycles {
			deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("tests failed, looping back to review-fix (cycle %d/%d): %v", reviewCycleCount+1, maxCycles, err))
			if _, statusErr := deps.UpdateTaskStatus(ctx, task.ID, models.TaskStatusReviewing); statusErr != nil {
				return nil, statusErr
			}
			return nil, workflow.ErrReviewFixLoop
		}
		deps.Log(ctx, task.ID, &jobID, "warn", fmt.Sprintf("tests failed and review-fix cycle limit reached (%d/%d), failing", reviewCycleCount, maxCycles))
		return nil, err
	}

	if deps.Wkspace != nil {
		if ws, errWS := deps.Wkspace.LoadTaskWorkspace(ctx, task); errWS == nil {
			for i := range ws.Repos {
				ws.Repos[i].Status.TestStatus = models.TestStatusPassed
			}
			_ = deps.Wkspace.SaveTaskWorkspaceMetadata(task, ws)
		}
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
	_ = deps.SaveArtifact(ctx, jobID, task.ID, workflow.StepTest, "test_output", out)
	return out, nil
}
