package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/steps"
	"github.com/auto-code-os/auto-code-os/server/pkg/attest"
)

// AttestationSignerAdapter implements steps.AttestationSigner on top of
// AttestationService, so the orchestrator/steps package (which only depends
// on narrow interfaces, never on internal/service directly) can sign
// per-commit attestations without importing this package's concrete types.
type AttestationSignerAdapter struct {
	svc *AttestationService
}

func NewAttestationSignerAdapter(svc *AttestationService) *AttestationSignerAdapter {
	return &AttestationSignerAdapter{svc: svc}
}

func (a *AttestationSignerAdapter) SignCommit(ctx context.Context, in steps.AttestationSignInput) error {
	var reviewedBy *attest.Actor
	if in.HasReviewedBy {
		reviewedBy = &attest.Actor{Provider: in.ReviewedByProvider, Model: in.ReviewedByModel}
	}
	_, err := a.svc.SignCommit(ctx, SignInput{
		RepoName:   in.RepoName,
		CommitHash: in.CommitHash,
		TaskID:     in.TaskID,
		JobID:      in.JobID,
		CodedBy: attest.Actor{
			Engine:   in.CodedByEngine,
			Provider: in.CodedByProvider,
			Model:    in.CodedByModel,
		},
		ReviewedBy: reviewedBy,
		PromptHash: in.PromptHash,
		Policy: attest.PolicySnapshot{
			Autonomy:      in.Autonomy,
			ReviewHarness: in.ReviewHarness,
			FixCyclesUsed: in.FixCyclesUsed,
		},
	})
	return err
}
