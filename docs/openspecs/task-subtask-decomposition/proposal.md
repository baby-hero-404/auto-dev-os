# Proposal: Task Sub-task Decomposition (Map-Reduce)

## Problem
A single heavy task drove a CLI agent run to ~356k input / ~54k output tokens in one turn and died at ~297s with `{"error": "timeout waiting for response"}`, exit status 1.

Confirmed by reading the code (not assumed):
- The string `"timeout waiting for response"` does not exist anywhere in this repo's Go source — it is emitted by the CLI subprocess itself (`claude`/`codex`/`agy`) or its underlying LLM SDK client, not by AutoCodeOS.
- AutoCodeOS's own timeouts were nowhere near 297s: `CLIProfile.TimeoutMinutes` is 30 min for all three built-in profiles (`server/pkg/models/cli_profiles.go`), and `defaultCLITimeout` (`server/internal/orchestrator/engine/cli.go`) is 3h.
- Separately (found while investigating, not the cause of this incident): `sandbox.timeout_minutes` had no default in `config.yaml`, so `Orchestrator.sandboxTimeout` silently fell back to a 5-minute struct-literal default for the *generic* sandbox-step path. Fixed in this pass by adding `timeout_minutes: 60` to `config.yaml`'s `sandbox:` block.

## What already exists (verified by reading the code, not assumed)
This is not a greenfield feature. `parent_task_id` linkage already ships:
- `Task.ParentTaskID *string` and `Task.SubTasks []Task` (`server/pkg/models/task.go:66,86`)
- `TaskService.ListSubTasks` / `TaskService.CreateSubTask` (`server/internal/service/task.go:326-337`) — a thin wrapper that sets `ParentTaskID` and calls the normal `Create` path
- The migration adding `parent_task_id` already exists (predates this proposal)

What does **not** exist, and is the actual scope of this proposal:
- Any automatic trigger that decides a task should be split (today, sub-tasks are only created if a caller explicitly calls `CreateSubTask` — nothing in `analyze.go` looks at size/complexity)
- Any orchestration of children as a sequence (dispatch order, resume-from-failed-child, parent status derived from child statuses) — today a parent with children runs its own CLI step exactly like any other task; children are inert unless something else drives them
- Any Reduce/aggregation step
- Any concept of an execution *attempt* separate from a `Task` row, a dependency graph between siblings, an explicit input/output contract per child, a `BLOCKED` parent state, or a `decomposition_mode` config knob

This proposal is additive orchestration logic on top of the existing linkage — it must not re-add `parent_task_id`/`SubTasks`/`CreateSubTask`, only build the decision/execution/aggregation layer around them.

## Goal
Heavy tasks succeed reliably without depending on any CLI tool's internal network timeout or a provider's per-turn output-token ceiling (typically 4k–8k, regardless of how long we're willing to wait).

## Success
A task that today requires ~356k input / ~54k output tokens in one turn completes end-to-end (across as many sub-task runs as needed) without operator intervention beyond the one-time split review, and no single sub-task run risks the provider's per-turn output-token ceiling.

## Assumptions
- Raising `API_TIMEOUT_MS`/equivalent CLI env vars (stopgap, see Non-goals) buys headroom for input-heavy-but-output-light turns; it does NOT help once required *output* exceeds the provider's hard per-turn token cap.
- The existing `Task`/`ParentTaskID` model can carry the new fields below without a breaking change — additive columns only.
- Analyze step (`orchestrator/steps/analyze.go`) is the right place to compute a split proposal — it already runs before any CLI turn and already produces `TaskAnalysis`.

## Decisions
- **Decompose before execution, not after failure.** Detect "this task is large" at planning time via a multi-factor **Complexity Score**, not a single input-token estimate: token estimate, affected-file count, dependency depth (`TaskDAG.DependsOn` already computed by analyze), instruction/deliverable count (e.g. "build schema + API + frontend" = 3 deliverables). A single-metric token threshold under- or over-triggers depending on task shape (a 200k-token single-file refactor and a 50k-token five-module feature are not equally risky); a composite score reflects that.
- **Execution attempts are a separate entity from the Task itself (`TaskAttempt`).** A `Task` (parent or child) identifies *what* the work is; a `TaskAttempt` records *one execution* of it (start/end time, exit status, tokens in/out, cost, which container/sandbox run). This separation is added in Phase 1, not deferred, because retrying a child (or the whole parent) must not be modeled as mutating the child `Task` row in place — that would destroy the audit trail of what was actually tried and lose the "which attempt failed at 297s" evidence this very incident needed.
- **Children share the parent's workspace lineage, not isolated per-child workspaces.** Reusing an isolated-per-task worktree per child (as a normal top-level task gets) means child 2 cannot see child 1's uncommitted-but-real changes without an explicit merge/handoff step, which reintroduces exactly the context-loss problem decomposition is meant to solve. Instead, children execute sequentially on the same branch/worktree as the parent, each committing its own changes before the next child starts — "continuous" execution, not "isolated-then-merged."
- **`decomposition_mode` is a config knob (`manual` | `auto` | `disabled`), default `manual`.** Splitting is not free (more orchestration, more LLM calls, more places to get boundaries wrong) and not every project/operator wants it. `manual` keeps today's default behavior of "propose, operator approves" for any project that hasn't opted in further; `auto` skips the review gate once split-quality is trusted; `disabled` keeps the pre-existing single-task path unconditionally, matching the feature flag's rollback story.
- **Reduce is deterministic (non-LLM) code in v1.** The Reduce step's job — union changed-file lists, aggregate test pass/fail counts, roll up cost/duration — is pure data aggregation, not judgment. Running it through an LLM would (a) reintroduce a context-size risk one level up and (b) make the parent's final status non-reproducible for the same child outcomes. An LLM may still be used *after* Reduce, off the critical path, purely for prose (e.g. a combined PR description) — but never to decide success/failure or to merge structured data.
- **Children declare a Contract, not just an instruction string.** Each `ChildTaskSpec` carries `input.previous_summary` (the prior sibling's structured `ChildTaskSummary`, not raw transcript) and `output_expected` (files/tests the child is expected to touch/pass). This makes a wrong split boundary detectable immediately (child 2 didn't touch a file it was contracted to touch) instead of surfacing only as a confusing downstream failure.
- **A failed child blocks the parent; it does not fail it.** `failed` on the parent implies "this work item did not happen" — but sibling children before the failure point did complete real, verified work. The parent instead enters `blocked` (new status, distinct from `failed`), naming the failing child, and remains resumable. This preserves the existing `failed`→re-enterable-anywhere transition set for genuinely-failed tasks while giving decomposed parents a status that means "some progress, stuck at child N" rather than conflating the two.

## Trade-offs
- **Gain:** No task is structurally capable of hitting a provider output-token ceiling or an unbounded single-turn duration; failures are isolated to one sub-task with a precise resume point; partial progress survives a crash; `TaskAttempt` gives durable forensics for incidents like this one going forward.
- **Lose:** More orchestration overhead (more container starts, more LLM calls, more inter-task coordination code, one more DB table); harder for a human to reason about "the task" as one linear log; shared-workspace sequential execution means children cannot run concurrently in v1 even though the dependency graph is captured (stored, not yet used for scheduling).

## Non-goals
- Fixing the CLI tools' own internal HTTP timeouts. That is a config stopgap (setting the CLI's documented network-timeout env var via each project's existing `CLIEngineConfig.Env`, `server/pkg/models/project.go:52`) applied *today*, independent of this spec.
- Auto-detecting the "right" split boundaries with no operator input when `decomposition_mode=manual` (the default) — `auto` mode is supported by the data model from Phase 1 but ships gated off by default.
- Parallel execution of children, even though `depends_on` is captured — v1 scheduling stays strictly sequential in dependency order.

## Out of Scope
- Concurrent/parallel sub-task execution — the `depends_on` graph exists so a future scheduler can use it, but this proposal does not build that scheduler.
- Cross-task shared context/caching beyond the per-child Contract's `previous_summary`.

## Impact

### Components
- `internal/orchestrator/steps/analyze.go` (complexity score, split proposal, contracts)
- `internal/orchestrator` (child dispatch order, `blocked` derivation, Reduce)
- `pkg/models/task.go` (new `Task` fields: `SequenceIndex`, `DecompositionMode`, `ComplexityScore`, `DependsOn`; new `TaskAttempt` type)
- `internal/service/task.go` (attempt creation on execution; child creation from an approved split reuses existing `CreateSubTask`, not a new code path)
- `web/.../tasks/[taskID]` (UI: show sub-task tree, aggregate status, split review)

### Files
See `tasks.md` for the phase-by-phase file list.

### Public API
- Task detail response gains: `subtasks []TaskSummary` (already exists via `SubTasks`, now populated by real orchestration), `decomposition_mode`, `complexity_score`, `depends_on []string`, `attempts []TaskAttemptSummary`.
- New `blocked` value for `Task.Status`, additive to `ValidTaskTransitions`.
- No existing field removed or renamed.

### Migration
- New table `task_attempts` (parent-referencing, one row per execution attempt of any `Task`, parent or child).
- New nullable columns on `tasks`: `sequence_index int`, `decomposition_mode text`, `complexity_score jsonb`, `depends_on text[]`.
- `parent_task_id` itself is NOT part of this migration — it already exists.

### Backward Compatibility
Tasks created before this feature have `decomposition_mode = NULL` (treated as `manual`'s pre-existing behavior: no auto-split, `CreateSubTask` still callable directly as it is today), `complexity_score = NULL`, `depends_on = '{}'`. Non-decomposed tasks take the same code path as before.
