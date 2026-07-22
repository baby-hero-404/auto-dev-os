# Implementation Notes: Definition-of-Ready Gate

Spec: `docs/openspecs/definition-of-ready-gate/`.

## Starting state

Most of this spec was already implemented under a prior, undocumented pass: `AnalyzeStep.applyAnalyzePolicy` (`server/internal/orchestrator/steps/analyze.go`) and `TaskService.Analyze`/`Clarify` (`server/internal/service/task.go`) already generate `clarification_questions` as part of the analyzer's structured response, persist them into `task.Clarifications` as `models.ClarificationRound`s, and pause the pipeline via `workflow.PauseError` when `policy.ShouldAutoApproveSpec` returns `TaskSpecStatusClarificationRequired`. This is functionally REQ-001/002/003(partial)/005/M01 — there was no separate `dor_check` step, `dor_check.md` prompt, or new DAG node to build.

## What was actually missing and added

1. **Round limit (REQ-003, second half)**: `policy.MaxClarificationRounds = 2` and `policy.IsDefinitionOfReadyBypassed(labels []string, priorClarificationRounds int) bool` (`server/internal/policy/review_policy.go`) — bypasses the clarification-required block once a task has already cycled through `MaxClarificationRounds` unanswered/still-insufficient rounds, to avoid deadlocking the pipeline forever on a task whose clarifications keep coming back insufficient.
2. **Hotfix bypass (REQ-004)**: the same `IsDefinitionOfReadyBypassed` also returns true for any task with a `hotfix` label (case-insensitive), regardless of round count.
3. **`models.TaskSpecStatusReadyWithWarnings = "ready_with_warnings"`** (`pkg/models/task.go`) — a new spec status distinct from `auto_approved`, so a task that skipped the DoR gate is auditable/distinguishable from one that was genuinely ready.
4. **Wiring**: both `AnalyzeStep.applyAnalyzePolicy` and `TaskService.Analyze` now compute `dorBypassed` from `(task.Labels, len(priorClarificationRounds))` before calling `ShouldAutoApproveSpec`; when bypassed, `hasClarifications` is forced to `false` for that call, and if the resulting `specStatus` would have been `TaskSpecStatusAutoApproved`, it's relabeled to `TaskSpecStatusReadyWithWarnings`. A `TaskSpecStatusPendingReview` result (e.g. from unrelated high-risk-domain review policy) is left untouched — a legitimate pending-review pause must still block regardless of the DoR bypass; only the auto-approve path gets the "proceeded with warnings" marker.

## Deviations from tasks.md / design.md

- **No dedicated `dor_check` step / DAG node (REQ-M01, tasks 1.2/1.6)**: the readiness check was already embedded in the existing `analyze` step before this pass; adding a separate node would mean re-validating the same fields a second time for no behavioral gain. Design.md's own CLI-mode section endorses embedding readiness as a precondition rather than a separate spawn — the same reasoning applies here since the logic already lived inline.
- **REQ-004b (CLI-mode LLM-unavailable fallback) skipped**: `cli_analyze.go` (the CLI-mode analyze step) never calls an API-native LLM for anything — it's a pure black-box CLI subprocess with no clarification-question generation at all. There is nothing to "fall back" from. Building speculative LLM dependency-injection wiring into `CLIAnalyzeStep` purely to guard a code path that doesn't exist yet is deferred until CLI-mode clarification generation is actually implemented.
- **Task 1.8 (UI: `ready_with_warnings` badge)**: skipped this pass. The existing clarifications answer UI is untouched; a distinct visual badge for `ready_with_warnings` would be a small follow-up if operators need to audit which tasks proceeded despite unresolved clarifications.

## Key files

- `server/internal/policy/review_policy.go` — `MaxClarificationRounds`, `IsDefinitionOfReadyBypassed`.
- `server/internal/orchestrator/steps/analyze.go` — `applyAnalyzePolicy` bypass wiring (api-native workflow path).
- `server/internal/service/task.go` — `Analyze` bypass wiring (the simpler non-workflow analyze path).
- `server/pkg/models/task.go` — `TaskSpecStatusReadyWithWarnings`.
- `server/internal/policy/review_policy_test.go` — `TestIsDefinitionOfReadyBypassed`.
- `server/internal/orchestrator/steps/analyze_step_test.go` — `TestAnalyzeStep_DefinitionOfReadyBypass_HotfixLabel`.
