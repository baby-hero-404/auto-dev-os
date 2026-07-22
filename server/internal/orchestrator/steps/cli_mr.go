package steps

import "github.com/auto-code-os/auto-code-os/server/internal/workflow"

// CLIMRStep reuses PRStep's push+merge-request logic for the CLI spec-first
// flow's final stage: the spec set (docs/openspecs/<slug>/) is part of the
// same worktree diff as the code changes, so reviewers see spec and
// implementation together in one merge request.
type CLIMRStep struct {
	*PRStep
}

func NewCLIMRStep(pr *PRStep) *CLIMRStep {
	return &CLIMRStep{PRStep: pr}
}

func (s *CLIMRStep) ID() string { return workflow.StepCLIMR }
