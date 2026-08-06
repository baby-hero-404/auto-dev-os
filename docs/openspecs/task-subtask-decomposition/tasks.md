# Implementation Map: Task Sub-task Decomposition

**Goal:** Let an oversized task split into sequential, independently-executed child tasks whose outcomes are aggregated back into the parent, instead of one CLI turn attempting the whole thing.
**Tech Stack:** Go (server), Postgres migration, existing orchestrator/task state machine, Next.js (task detail UI).
**Already shipped, do not re-implement:** `Task.ParentTaskID`/`Task.SubTasks` (`server/pkg/models/task.go:66,86`), `TaskService.ListSubTasks`/`CreateSubTask` (`server/internal/service/task.go:326-337`), the migration adding `parent_task_id`. This plan only adds the automatic decision/execution/aggregation layer around that existing linkage.

---

## Phase 0: Immediate stopgap (no schema change, ship independently of the rest)

### CLI network-timeout config guidance
**Why:** Buys headroom today for input-heavy turns while this feature is built — does not fix output-token-ceiling failures, but costs nothing to apply.

**Depends on:** None.

**Files:**
- None (config-only, via existing `Project.CLIEngineConfig.Env`, `pkg/models/project.go:52`)

**Changes:**
- [x] Document, per CLI profile, which env var (if any) raises its internal HTTP client timeout (verify against each CLI's own `--help`/docs — do not guess)
- [x] Add the verified var(s) to the affected project's `cli_engine_config.env` via the existing API/UI

**Verify:**
- [x] A previously-failing large-input/moderate-output task no longer dies at ~297s

---

## Phase 1: Data model (TaskAttempt + decomposition fields)

### `TaskAttempt` table and model
**Why:** Separates "what happened on one execution try" from "what this task is." Needed before any dispatch/retry logic can record attempt-level forensics (tokens, cost, exit status) instead of overwriting them on the `Task` row.

**Depends on:** None (independent of the existing `parent_task_id` linkage).

**Files:**
- `server/migration/000024_add_task_attempts.up.sql` / `.down.sql`
- `server/pkg/models/task_attempt.go` (new)
- `server/internal/repository/task_attempt_repo.go` (new — Create, ListByTaskID, GetLatestByTaskID)

**Changes:**
- [x] `task_attempts` table: `id uuid pk`, `task_id uuid references tasks(id)`, `attempt_number int`, `started_at timestamptz`, `finished_at timestamptz null`, `exit_status int null`, `tokens_in int null`, `tokens_out int null`, `cost_usd numeric null`, `sandbox_ref text null`
- [x] `TaskAttempt` Go struct mirroring the table
- [x] Repo methods used by later phases; no orchestrator wiring yet in this phase

**Verify:**
- [x] Migration applies cleanly on top of the latest existing migration
- [x] `go test ./pkg/models/... ./internal/repository/...` passes

### Decomposition fields on `Task`
**Why:** `sequence_index`/`decomposition_mode`/`complexity_score`/`depends_on` are needed by Phase 2's split proposal and Phase 3's dispatch order.

**Depends on:** None.

**Files:**
- `server/migration/000025_add_task_decomposition_fields.up.sql` / `.down.sql`
- `server/pkg/models/task.go`

**Changes:**
- [x] Add nullable `sequence_index int`, `decomposition_mode text`, `complexity_score jsonb`, `depends_on text[]` columns (additive)
- [x] Add corresponding `Task` struct fields
- [x] Add `TaskStatusBlocked = "blocked"` constant; extend `ValidTaskTransitions` — reachable from the decomposed-parent running state, transitions to itself (retry) or to the parent's normal forward states once unblocked

**Verify:**
- [x] Migration applies cleanly on top of `000024`
- [x] Existing task queries (list/get) unaffected for rows with all new columns NULL
- [x] `workflow.ValidateTaskTransition` rejects `blocked` as a source for a non-decomposed task's existing transitions (no accidental new edge)

---

## Phase 2: Analyze-time complexity score and split proposal

### Complexity Score
**Why:** A single token threshold under/over-triggers depending on task shape; a composite score reflects actual split risk.

**Depends on:** Phase 1 (decomposition fields).

**Files:**
- `server/internal/orchestrator/steps/analyze.go`
- `server/pkg/config/config.go` (new `execution.decomposition_threshold` config, and `execution.decomposition_mode_default = "manual"`)

**Changes:**
- [x] Compute `ComplexityScore{Tokens, Files, DependencyDepth, DeliverableCount, Total}` alongside existing `buildTaskAnalysis` estimate — reuse `TaskDAG.DependsOn` (already computed) for dependency depth
- [x] Store the score on `Task.ComplexityScore` regardless of whether a split is proposed (needed for `task.tokens.before_split` metric and future threshold tuning)
- [x] When `Total` exceeds the configured threshold AND the task's/project's `decomposition_mode` (resolved: task override, else project default, else `manual`) is not `disabled`, produce an ordered `[]ChildTaskSpec` with a Contract per child (`input.previous_summary` nil for child 0, `output_expected` from analyze's affected-files estimate)

**Verify:**
- [x] A task whose score is under threshold takes the exact same path as before this feature (no behavior change)
- [x] A task over threshold with `decomposition_mode=manual` produces a reviewable split proposal and does not execute until approved/rejected
- [x] A task over threshold with `decomposition_mode=disabled` proceeds as a normal single task, score still recorded

---

## Phase 3: Sequential child execution + resume

### Child dispatch on shared workspace lineage
**Why:** Reuse existing per-task execution/retry/checkpoint machinery; children commit sequentially onto the parent's own branch instead of isolated-then-merged worktrees, so each child sees the real prior state.

**Depends on:** Phase 1, Phase 2.

**Files:**
- `server/internal/service/task.go` (split approval → child creation, reusing existing `CreateSubTask`, setting `SequenceIndex`/`DependsOn`)
- `server/internal/orchestrator/orchestrator.go` (dispatch next unstarted/failed child in `SequenceIndex` order on the parent's workspace; parent status derived from children — `blocked` on child failure per specs.md)
- `server/internal/orchestrator/engine/cli.go` (wrap each child execution attempt in a `TaskAttempt` create/finalize)

**Changes:**
- [x] On split approval (manual) or automatically (auto mode), create one `Task` row per `ChildTaskSpec` via existing `CreateSubTask`, additionally setting `SequenceIndex` and resolved `DependsOn` (spec indices → task IDs)
- [x] Parent task never runs a CLI step directly once it has children — it only tracks/aggregates
- [x] Each child execution creates a `TaskAttempt` at start, finalizes it (exit status, tokens, cost, duration) at end
- [x] A child failure sets parent status to `blocked`, recording the failing child ID + `SequenceIndex` (not `failed`)
- [x] Retrying a `blocked` parent creates a new `TaskAttempt` for the failed child only and resumes dispatch from there — already-succeeded siblings are untouched
- [x] Children execute against the parent's existing workspace/worktree (no new per-child worktree allocation)

**Verify:**
- [x] Killing the process mid-way through child 2 of 3, then retrying the parent, does not re-run child 1 and creates exactly one new `TaskAttempt` for child 2
- [x] A child's own existing timeout/retry behavior (30min/3h CLI timeouts, idle-timeout watchdog) is untouched
- [x] Child 2's workspace contains child 1's committed changes without any explicit "merge" step

---

## Phase 4: Reduce aggregation (deterministic)

### Summary-only rollup
**Why:** Prevent the Reduce step from recreating the original context-blowup one level up; keep the parent's terminal status reproducible.

**Depends on:** Phase 3.

**Files:**
- `server/internal/orchestrator/steps/reduce.go` (new, mirroring existing step file conventions)
- `server/pkg/models/task.go` (`ChildTaskSummary` type: changed files, test pass/fail counts, cost/duration, one-line outcome, `ContractDeviation`)

**Changes:**
- [x] Plain Go aggregation (no LLM call) after all children reach a terminal success state: union changed files, sum test pass/fail counts, sum cost/duration
- [x] Detect contract deviations (child's actual changed files vs. its `output_expected`) and attach as a warning on `ChildTaskSummary`, surfaced but non-blocking
- [x] Emit `task.tokens.after_split`, `task.duration.actual`, `task.cost.saved` metrics from the aggregated `TaskAttempt` data plus the stored `task.tokens.before_split`/`task.duration.single_estimate` from Phase 2
- [x] Any post-Reduce LLM step (e.g. combined PR description) consumes only the aggregated `ChildTaskSummary` list, never raw child transcripts

**Verify:**
- [x] Parent task's final changed-file list equals the union of all children's changed files
- [x] No child's raw CLI stdout/stderr is ever passed as input to another step
- [x] `task.cost.saved` is computed and non-nil on every completed decomposed parent

---

## Phase 5: UI

### Sub-task tree + split review
**Why:** The operator-review gate (specs.md) requires a place to approve/edit the proposed split under `manual` mode, and the existing task detail page needs to show decomposed progress and `blocked` state.

**Depends on:** Phase 2, Phase 3.

**Files:**
- `web/src/app/projects/[id]/tasks/[taskID]/` (new sub-task list/tree component)
- `web/src/lib/api/projects.ts` or equivalent task API client (new `decomposition_mode`, `complexity_score`, `depends_on`, `attempts` fields)

**Changes:**
- [x] Task detail page shows child task list with individual statuses (including per-child `ContractDeviation` warning) when `subtasks` is present
- [x] Split-proposal review UI (manual mode only): accept as-is / edit / reject
- [x] `blocked` status renders distinctly from `failed` (different badge/color), with an affordance to edit the failing child's instructions before retry
- [x] Project/task settings expose `decomposition_mode` selector (`manual`/`auto`/`disabled`)

**Verify:**
- [x] A parent task with 3 children shows all 3 statuses updating live as they execute
- [x] Rejecting a proposed split falls through to the existing single-task execution UI unchanged
- [x] A `blocked` parent is visually distinguishable from a `failed` one and shows which child/sequence_index is stuck

---

## Rollback Plan

### Feature Flags
- [x] `execution.decomposition_mode_default` config — set project/task `decomposition_mode=disabled` to opt any scope out entirely; all such tasks take the pre-existing single-task path regardless of Complexity Score

### Database Rollback
- [x] Reverse migrations: `migrate down` to before `000024` — safe since all new columns/tables are additive and unused once `decomposition_mode=disabled`
- [x] No data backfill needed; nothing depends on the new columns being populated

### Safe Deploy
- [x] Ship Phase 0 (config stopgap) immediately, independent of the rest
- [x] Ship Phases 1–4 with `decomposition_mode` defaulting to `disabled` at the org/project level until validated
- [x] Enable `manual` mode for one project first, monitor `task.decomposition.child_failure_rate` and `task.cost.saved`, before enabling broadly; `auto` mode stays opt-in per project even after `manual` is broadly enabled
