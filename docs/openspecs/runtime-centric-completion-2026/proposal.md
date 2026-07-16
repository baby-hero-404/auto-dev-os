# Proposal: Runtime-Centric Completion 2026

## Why

An external "Architecture Enhancement Report" reviewed this orchestrator and recommended moving
from a **Prompt-Centric Execution Model** to a **Runtime-Centric Execution Model**: the runtime
owns execution state, repository mapping, boundaries, acceptance criteria and tool contracts; a
Prompt Compiler renders deterministic prompts from that state; the LLM only reasons and emits
tool calls within a node.

Cross-checking the report's 12 findings against the codebase (not just the report's assumptions)
shows most of that direction was **already decided and mostly built** under
`docs/openspecs/execution-semantics-2026/`:

- Execution IR + Prompt Compiler — Task 1.1, `✅ Fully Implemented` (`pkg/models/ir.go`,
  `internal/prompts/compiler.go`).
- Intent Resolver — Task 1.2, `✅ Fully Implemented` (`internal/orchestrator/steps/intent_resolver.go`).
- Node State Machine + Phase Budgets — Task 2.1, `✅ Fully Implemented and Integrated`
  (`internal/orchestrator/llmrunner/statemachine.go`, `statemachineloop.go`).
- ExecutionSnapshot replacing Git-based salvage — Task 3.1, `✅ Fully Implemented`.
- Flag-gated migration off `RunToolLoop` — Task 2.2, `⚠️ Mostly Implemented — parallel telemetry
  active, FSM wired, pending release cycle to delete legacy loop`. `execution.state_machine_enabled`
  defaults to `false` (`pkg/config/config.yaml`), so production still runs the legacy
  `RunToolLoop`/`builder.go` path today; the FSM only observes and logs
  `[TELEMETRY-VIOLATION]` warnings without enforcing (`runner.go:305-319`, confirmed against a
  live task trace where every violation was logged but never blocked).

This proposal does **not** re-propose what's already shipped. It closes the one open task (2.2)
with a concrete, evidence-based rollout gate — today's spec says "pending release cycle" without
defining what that means or how to measure it — and adds the three findings from the report that
have **no existing coverage anywhere** in this repo's openspec history: review evaluating intent
rather than only a diff (Finding 2), execution-unit size enforcement (Finding 6), and a semantic
hash decoupled from prompt wording (Finding 12).

It also takes on the report's Finding 1/9 position — that the LLM should not be the source of
truth for `target_files` — as a **deliberate reversal** of a shipped, tested design choice
(Task 1.2's Intent Resolver today matches against `analysis.AffectedFiles`, which the Planner LLM
supplies, and only takes over when the LLM leaves `target_files` empty:
`intent_resolver.go:97-99`; `analyze.go:322-332` currently *requires* `target_files` non-empty).
This is flagged as higher-risk than the other issues below because it changes already-tested,
already-shipped behavior rather than filling a gap.

## What Changes

### Issue 1: State Machine Rollout Has No Defined Gate
- Define a measurable go/no-go gate for flipping `state_machine_enabled` to `true`, using data
  the shadow FSM already collects (`[TELEMETRY-VIOLATION]` warn-log rate) instead of a vague
  "release cycle."
- Define the retirement sequence for `RunToolLoop` (`llmrunner/toolloop.go`) and the legacy
  prompt assembler (`internal/prompts/builder.go`/`assembler.go`, ~1300 lines) once the gate is
  met — these two only stay alive today because the flag is off.

### Issue 2: Intent Resolver Becomes the Authoritative Target Source (reversal)
- `analyze.go` stops requiring `target_files` as a hard validation failure; the LLM emits
  `Objective`/`Intent.Capability` as the primary signal.
- `intent_resolver.go`'s `ResolveIntent` runs unconditionally (not just when `target_files` is
  empty) and becomes the authoritative producer of `ExecutionIRTargets`. Any LLM-suggested
  `target_files` is folded in as an additional matching signal, not trusted verbatim.
- Existing resolver tests (`TestResolveIntent_Resolvable/Ambiguous/Unresolvable`) keep their
  contract; `analyze_step_test.go`'s "target_files required" assertion is removed and replaced
  with a resolver-authoritative assertion.

### Issue 3: Review Evaluates a Diff Only, Not the Unit's Intent
- `review.go` already injects Acceptance Criteria, Execution Boundaries and `TasksMD`
  alongside the raw diff (`review.go:257-275`) — this is closer to the report's Finding 2 than the
  report assumed. The actual gap: the execution units' own `Objective` fields (their stated
  goals) are never included, so the reviewer infers intent only from AC/boundaries, not from why
  each unit exists.
- Render every frozen unit's `Objective` into the review instruction alongside the existing
  AC/boundary blocks. No model change needed — `FrozenContext` already snapshots
  `ExecutionUnits []ExecutionUnit` (`task.go:235`), and the review diff is the merged result of
  all units, so all objectives are rendered, keyed by unit ID.

### Issue 4: Execution Units Have a Declared but Unenforced File-Count Budget
- `ExecutionConstraints.MaxFiles` exists and feeds `PhaseBudgets` (`plan.go:100-116`), but nothing
  rejects or splits a unit whose `TargetFiles`/resolved targets exceed it — the field is advisory
  only today.
- At `PLAN_READY`, after Intent Resolver produces the final target list, enforce `MaxFiles` as a
  hard boundary: a unit resolving to more files than its own declared `MaxFiles` fails at
  `PLAN_READY` (same failure surface Task 1.2 already uses for unresolvable intents), not
  mid-`IMPLEMENTATION`.

### Issue 5: Prompt Hash Cannot Detect "Nothing Semantically Changed"
- `PromptHash` (`pkg/models/ir.go:134`) hashes the fully-rendered prompt text and is already used
  for replay/resume integrity (`llm_step.go:107-109`) — it changes on any wording change even when
  the underlying execution state is identical.
- Add a `SemanticHash` computed from `{NodeID, Intent, Acceptance, resolved TargetFiles,
  Constraints}` — the execution-state fields, not their rendering — so a resume/retry can detect
  "nothing that matters changed" independent of prompt-text drift (e.g. a Prompt Compiler
  wording tweak no longer forces a full re-reason).

## Capabilities

### New Capabilities
- Measurable rollout gate for `state_machine_enabled` based on existing shadow-FSM telemetry.
- `MaxFiles` enforcement at `PLAN_READY`.
- `SemanticHash` on `ExecutionIR` / `ExecutionSnapshot`, independent of `PromptHash`.

### Modified Capabilities
- `steps/analyze.go` — `target_files` becomes optional input, no longer a required field.
- `steps/intent_resolver.go` — `ResolveIntent` runs unconditionally and is authoritative;
  LLM-suggested paths are corroboration-gated signal, not a verbatim-trusted fallback.
- `steps/review.go` — instruction now includes the frozen execution units' objectives.

### Removed Capabilities (post rollout-gate only, not in this change)
- `llmrunner/toolloop.go` (`RunToolLoop`) and the legacy prompt assembler path
  (`internal/prompts/builder.go`/`assembler.go`) are marked for removal once Issue 1's gate is
  met for one full release cycle — tracked here, executed as a follow-up once the gate passes.

## Impact

| Area | Files Affected |
|------|----------------|
| Rollout gate / telemetry | `server/internal/orchestrator/llmrunner/statemachineloop.go`, `server/internal/orchestrator/llmrunner/runner.go`, `server/pkg/config/config.go`, `server/pkg/config/config.yaml` |
| Intent ownership reversal | `server/internal/orchestrator/steps/analyze.go`, `server/internal/orchestrator/steps/intent_resolver.go`, `server/internal/orchestrator/steps/plan.go` (owns the disagreement log call), `server/internal/orchestrator/steps/analyze_step_test.go`, `server/internal/orchestrator/steps/intent_resolver_test.go`, `server/internal/orchestrator/steps/plan_test.go` |
| Rollout gate repositories | `server/internal/repository/task.go`, `server/internal/repository/workflow.go`, `server/internal/repository/workflow_test.go` |
| Review intent injection | `server/internal/orchestrator/steps/review.go`, `server/internal/orchestrator/steps/review_test.go` |
| Execution unit sizing | `server/internal/orchestrator/steps/intent_resolver.go` (enforcement lives in `ResolveExecutionIRTargets`; `Constraints.MaxFiles` already exists in the model) |
| Semantic hash | `server/pkg/models/ir.go`, `server/internal/orchestrator/llmrunner/statemachineloop.go`, `server/internal/orchestrator/checkpoint/` |
| Legacy retirement (follow-up, gated) | `server/internal/orchestrator/llmrunner/toolloop.go`, `server/internal/prompts/builder.go`, `server/internal/prompts/assembler.go` |
