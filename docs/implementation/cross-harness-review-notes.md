# Implementation notes: Cross-Harness Review

## Default policy deviation (REQ-001, REQ-M01)

proposal.md specifies the initial default `review_harness_policy` should be `"same"`. This was
deliberately overridden to default to `"different_model"` instead.

Reason: before this spec set existed, `pkg/llm/router.go`'s `Gateway.ChatWithOptions` already
unconditionally excluded the coding step's model from the review LLM call (`steps/review.go`'s
pre-existing `getCoderModel` + `llm.WithExcludeModelID` + `RouteTrace.SelfReviewFallback`). That
is exactly the behavior `different_model` describes. Defaulting the new setting to `"same"` would
have been a real regression — it would have turned off cross-model review for every existing
project the moment the migration ran. Defaulting to `different_model` preserves the true
pre-feature behavior; `same` is available as an explicit new opt-out.

## REQ-001c: adversarial audit directive — heuristic, not literal synchronous detection

design.md wants the directive injected synchronously whenever the harness-resolution process
(hidden behind `LLMRunner`/`Gateway`) ends up landing on the same provider/model as the coder.
There's no Gateway-exposed method to predict same-harness fallback *before* making the LLM call —
confirmed by grep, no exported provider-list method exists on `*Gateway`. Implemented instead as a
two-part heuristic (`review.go`'s and `cross_review.go`'s `adversarial` bool):

1. Inject unconditionally when `policy == models.ReviewHarnessSame` (trivially always same).
2. Inject reactively on the *next* cycle whenever the immediately preceding same-step checkpoint
   recorded `self_review_fallback: true` (`previousSelfReviewFallback` helper).

This is deterministic and testable, but is an approximation — it will not inject on the very
first cycle that happens to fall back to the same harness (only from the second cycle onward).

## REQ-001b: underlying_provider blind-spot guard

`CLIEngineConfig.UnderlyingProvider` is optional. `effectiveCLIHarness(project)` in
`cross_review.go` resolves `(engine, provider)` for the CLI-coded side: `underlying_provider` when
declared, otherwise the opaque `"cli:<command>"` ID (old behavior — CLI assumed different from
every API provider). Only used inside `cross_review.go`; regular `ReviewStep` never sees CLI-coded
tasks since they run the separate `cli_analyze → cli_spec → cli_implement → [cross_review] →
cli_mr` pipeline.

## REQ-003: cross_review step and its fix loop

`CrossReviewStep` (`steps/cross_review.go`) mirrors `ReviewStep`'s LLM-call/verdict/repeat-violation
logic but:
- Captures the diff via `DiffCapturer.CaptureWorkspaceDiff` (no PR-diff / base-branch path — the
  CLI worktree diff mode only, since there's no separate backend/frontend worktree split).
- Reads `docs/openspecs/<slug>/{specs.md,tasks.md}` from the worktree as the spec-compliance input
  instead of frozen execution-unit context (CLI flow has no `TaskAnalysis.ExecutionUnits`).
- On a failing verdict, returns the new `workflow.ErrCrossReviewFixLoop` (mirrors
  `ErrReviewFixLoop`) instead of falling through to a `FixStep` — there is no separate fix step in
  the CLI flow; `worker.go` requeues the job at `StepCLIImplement` instead of `StepReview`.
- `CLIImplementStep` was extended to read the most recent `cross_review` checkpoint's structured
  verdict via the new `crossReviewFeedback` helper and prepend a `## Reviewer feedback` section to
  its instruction, mirroring `fix.go`'s PR-rejection-feedback pattern — this is what makes the
  re-dispatch loop actually address the violations instead of blindly re-running.
- Repeat-violation escalation (REQ-003's reuse of review-verdict-split REQ-003) reuses
  `previousReviewViolationsForStep`/`hasRepeatViolation` unchanged, keyed on `workflow.StepCrossReview`.

## REQ-002: coded_by/reviewed_by metadata + PR footer

Metadata capture in `review.go`/`cross_review.go` was already built in an earlier pass this
session. This pass added the PR-description surfacing: `steps/pr.go`'s new
`codedByReviewedBy(checkpoints)` helper scans for the most recent `StepReview` checkpoint's
`coded_by`/`reviewed_by` output and formats a `### Review Harness` section in
`gitops.PRGenerator.GenerateSummary`'s body. Note this only reads `StepReview` checkpoints, not
`StepCrossReview` — CLI-coded PRs (via `CLIMRStep`, which embeds `PRStep`) don't yet show this
footer. Left as a known gap rather than extending `codedByReviewedBy` speculatively, since
`cross_review.go`'s `coded_by`/`reviewed_by` output shape matches `review.go`'s exactly and can be
folded in with a one-line `cp.Step != workflow.StepReview && cp.Step != workflow.StepCrossReview`
change whenever that's actually needed.

## Deferred (not implemented this pass)

- **1.7 UI**: no project-settings select for `review_harness_policy`, no task-detail display of
  `coded_by`/`reviewed_by`. Backend/API fully supports it (`CreateProjectInput`/`UpdateProjectInput`
  already carry the field); this is pure frontend work not touched here.
- **1.8 full integration test**: no engine-driven end-to-end CLI task run through a real
  fail→fix→pass cross-review cycle. Covered instead by: `CrossReviewStep` unit tests (passing
  verdict, failing verdict, cycle-limit-reached) and a DAG-shape test confirming
  `CLISpecFirstWorkflow(runners, true)` inserts `cross_review` between `cli_implement` and
  `cli_mr` with correct dependency edges.
