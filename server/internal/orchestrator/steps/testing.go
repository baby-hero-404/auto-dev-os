package steps

import (
	"context"
	"fmt"
	"strings"

	orchtester "github.com/auto-code-os/auto-code-os/server/internal/orchestrator/tester"
	"github.com/auto-code-os/auto-code-os/server/internal/workflow"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

// TestStep implements Step for the test phase.
type TestStep struct {
	rt          StepRuntime
	status      StatusUpdater
	sandbox     SandboxRunner
	workspace   WorkspaceLoader
	projects    ProjectReader
	checkpoints CheckpointReader
	artifacts   ArtifactSaver
	log         Logger
}

func NewTestStep(
	rt StepRuntime,
	status StatusUpdater,
	sandbox SandboxRunner,
	workspace WorkspaceLoader,
	projects ProjectReader,
	checkpoints CheckpointReader,
	artifacts ArtifactSaver,
	log Logger,
) *TestStep {
	return &TestStep{
		rt:          rt,
		status:      status,
		sandbox:     sandbox,
		workspace:   workspace,
		projects:    projects,
		checkpoints: checkpoints,
		artifacts:   artifacts,
		log:         log,
	}
}

func (s *TestStep) ID() string                         { return workflow.StepTest }
func (s *TestStep) StatusOnResume(_ StepResult) string { return models.TaskStatusTesting }

func (s *TestStep) Execute(ctx context.Context, stepCtx workflow.StepContext) (StepResult, error) {
	if s.status != nil {
		if _, err := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusTesting); err != nil {
			return nil, err
		}
	}
	if s.sandbox == nil {
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "info", "No sandbox runner available, skipping tests")
		return StepResult{"status": "skipped", "reason": "no_sandbox"}, nil
	}
	script := orchtester.FullVerificationScript()
	out, err := s.sandbox.RunCommand(ctx, s.rt.Task, s.rt.Agent, workflow.StepTest, script)
	if err != nil {
		if s.workspace != nil {
			if ws, errWS := s.workspace.LoadTaskWorkspace(ctx, s.rt.Task); errWS == nil {
				for i := range ws.Repos {
					ws.Repos[i].Status.TestStatus = models.TestStatusFailed
				}
				_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
			}
		}

		// Spec 5.7 Step 9: on test failure, loop back to fix within cycle limits.
		maxCycles := 3
		if s.projects != nil {
			if p, pErr := s.projects.GetByID(ctx, s.rt.Task.ProjectID); pErr == nil && p.MaxReviewFixCycles > 0 {
				maxCycles = p.MaxReviewFixCycles
			}
		}
		reviewCycleCount := 0
		if s.checkpoints != nil {
			reviewCycleCount = s.checkpoints.CountSuccessful(ctx, s.rt.Task.ID, workflow.StepReview)
		}
		if s.rt.Task.Complexity != models.TaskComplexityEasy && reviewCycleCount < maxCycles {
			s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("tests failed, looping back to review-fix (cycle %d/%d): %v", reviewCycleCount+1, maxCycles, err))
			if s.status != nil {
				if _, statusErr := s.status.UpdateTaskStatus(ctx, s.rt.Task.ID, models.TaskStatusReviewing); statusErr != nil {
					return nil, statusErr
				}
			}
			return nil, workflow.ErrReviewFixLoop
		}
		s.log.Log(ctx, s.rt.Task.ID, &s.rt.JobID, "warn", fmt.Sprintf("tests failed and review-fix cycle limit reached (%d/%d), failing", reviewCycleCount, maxCycles))
		return nil, err
	}

	if s.workspace != nil {
		if ws, errWS := s.workspace.LoadTaskWorkspace(ctx, s.rt.Task); errWS == nil {
			for i := range ws.Repos {
				ws.Repos[i].Status.TestStatus = models.TestStatusPassed
			}
			_ = s.workspace.SaveTaskWorkspaceMetadata(s.rt.Task, ws)
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
	if s.artifacts != nil {
		_ = s.artifacts.SaveArtifact(ctx, s.rt.JobID, s.rt.Task.ID, workflow.StepTest, "test_output", out)
	}
	return out, nil
}
