# Implementation Notes: Review Verdict Split

Spec: `docs/openspecs/review-verdict-split/`.

## Starting state

`ReviewStep` (`server/internal/orchestrator/steps/review.go`) only ever produced a single implicit verdict: `ParseReviewFindings` extracted a flat list of `models.ReviewFinding` from whatever shape the LLM returned (`findings`/`array`/single-object), and `hasActionableFindings` decided fix-vs-test purely from severity/`requires_fix`. There was no spec-vs-quality distinction and no escalation mechanism for repeated failures.

## What was added

1. **Structured verdict types** (`pkg/models/task.go`): `SpecViolation`, `QualityIssue`, `ReviewVerdict` (`{spec_compliance, code_quality, summary}`), matching design.md's schema exactly.
2. **Prompt** (`internal/prompts/steps/review.md`): added an "Output format" section specifying the 2-verdict JSON schema and what each axis covers. Regenerated the golden prompt snapshot (`internal/prompts/testdata/golden/review.golden`).
3. **Parser + fallback** (`review.go`): `ParseReviewVerdict(parsed map[string]any) (models.ReviewVerdict, bool)` — modeled on `analyze_parser.go`'s tolerant style; returns `ok=false` when neither `spec_compliance` nor `code_quality` is present, in which case `Execute` falls back to the pre-existing `ParseReviewFindings`/`hasActionableFindings` path unchanged (REQ-001 fallback, REQ-M01 backward compatibility).
4. **Routing** (`review.go` `Execute`/`StatusOnResume`): `hasFindings := specFail || qualityFail` (both axes independently checked); `nextStatus` is `fixing` if either fails, else `testing`. `StatusOnResume` mirrors the same logic when reading a checkpointed `review_verdict` back after a resume.
5. **Violation persistence & repeat-detection**: the verdict is serialized into `StepResult["review_verdict"]`, which flows into the step's checkpoint `state.output` via the orchestrator's existing checkpoint machinery (no new persistence layer needed). `previousReviewViolations` reads the most recent prior `StepReview` checkpoint's `output.review_verdict.spec_compliance.violations` via the already-injected `CheckpointLister`. `tokenSetOverlap`/`hasRepeatViolation` implement the design's "lowercase, trim, Jaccard token-set overlap ≥ 0.6" fuzzy match.
6. **Escalation pause** (REQ-003): when `spec_compliance=fail` and the current cycle's violations repeat-match the previous cycle's, `Execute` sets the task status to `models.TaskStatusHumanReview` and returns `workflow.PauseError{Step: workflow.StepReview, Reason: "awaiting_review_escalation: ..."}` — reusing the exact same pause mechanism already used by `analyze.go`, `merge.go`, and `cli_spec.go` (confirms the roadmap's "Pause/resume helper dùng chung" note: no shared helper was ever extracted because every set independently found `workflow.PauseError` sufficient).
7. **Fix instruction assembly** (REQ-002/004): `buildVerdictFixInstruction` (`coding_instruction.go`, per design.md's explicit file pointer) renders `## Spec violations (MUST fix first)` followed by `## Quality issues` from the structured verdict. `fix.go` prefers this over the legacy flat `findingsJSON` whenever `review_verdict` is present on the review step's output; the legacy path is otherwise untouched, so quality-only failures (or old-format reviews) still work exactly as before, and `MaxReviewFixCycles` counting is unaffected since it's driven by `reviewCycleCount`, not by which instruction-assembly path ran.

## Deviations from tasks.md / design.md

- **No new `TaskStatus`/DAG state for escalation**: rather than inventing a distinct `awaiting_review_escalation` task status with dedicated continue/cancel resume actions, the escalation reuses `TaskStatusHumanReview` (whose resume paths already exist in the UI/API) and encodes the escalation reason in the `PauseError.Reason` string. This mirrors the same "reuse before building new state machinery" judgment call made in `definition-of-ready-gate`.
- **UI badges (task 1.7 / REQ-005) skipped this pass**: the backend now always emits `review_verdict` on the review checkpoint output when the LLM returns the structured schema, so a frontend follow-up (spec/quality badges + expandable violation/issue lists) can be built without further backend work. Old checkpoints without `review_verdict` simply have no field to render — no crash, no migration needed (REQ-M01 is satisfied at the data level already).
- **Routing logic lives in `review.go`, not `worker.go`**: design.md sketches the routing as if it lived in `worker.go`, but `worker.go` only handles the review→fix loop-back (`ErrReviewFixLoop`) and job-status bookkeeping; the actual spec-fail/quality-fail/repeat-violation decision was already made per-cycle inside `ReviewStep.Execute` (which is where `hasActionableFindings`-based routing already lived before this change), so the new logic was added there for consistency rather than relocated to worker.go.

## Key files

- `server/pkg/models/task.go` — `SpecViolation`, `QualityIssue`, `ReviewVerdict`.
- `server/internal/prompts/steps/review.md` — 2-verdict output schema instruction.
- `server/internal/orchestrator/steps/review.go` — `ParseReviewVerdict`, `verdictAsMap`, `tokenSet`/`tokenSetOverlap`/`hasRepeatViolation`, `previousReviewViolations`, `Execute`/`StatusOnResume` routing + escalation.
- `server/internal/orchestrator/steps/coding_instruction.go` — `buildVerdictFixInstruction`.
- `server/internal/orchestrator/steps/fix.go` — wiring `buildVerdictFixInstruction` ahead of the legacy `findingsJSON` path.
- Tests: `server/internal/orchestrator/steps/review_test.go`, `server/internal/orchestrator/steps/fix_test.go`.
