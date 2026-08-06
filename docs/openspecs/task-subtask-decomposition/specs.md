# Expected Behavior: Task Sub-task Decomposition

## Scenario: Analyze step computes a Complexity Score
**When:**
- Analyze runs on any task (existing `TaskService.Analyze` → `buildTaskAnalysis` path)

**Then:**
- A `ComplexityScore` is computed from: estimated input tokens, affected-file count, dependency depth (from the already-computed `TaskDAG.DependsOn`), instruction/deliverable count (heuristic: count of independent objectives in the description, e.g. "schema + API + frontend" = 3)
- If the project's `decomposition_mode` is `disabled`, the score is still computed and stored (for future tuning/telemetry) but never triggers a split proposal
- If the score exceeds the configured threshold and `decomposition_mode` is `manual` or `auto`, analyze produces an ordered `[]ChildTaskSpec`, each carrying a Contract: `input.previous_summary` (nil for the first child) and `output_expected` (files/tests the child is expected to touch/pass)

## Scenario: Split proposal under `decomposition_mode=manual` (default)
**When:**
- A split proposal is produced and `decomposition_mode=manual`

**Then:**
- The parent task enters `planning_split` status, surfaced in the UI before any child task runs
- Operator may accept as-is, edit (reorder, merge, drop children, edit contracts), or reject
- Rejecting falls through to the existing single-task path unchanged — decomposition is never forced

## Scenario: Split proposal under `decomposition_mode=auto`
**When:**
- A split proposal is produced and `decomposition_mode=auto`

**Then:**
- The parent skips `planning_split` and proceeds directly to child creation with the proposed contracts — no operator review gate
- The proposal is still recorded on the parent's `Analysis` for later audit, exactly as a manual proposal would be, just not blocking on approval

## Scenario: Split approved (manual) or auto-proceeds (auto)
**When:**
- Operator confirms the proposed child list, or `decomposition_mode=auto` proceeds automatically

**Then:**
- One child `Task` row is created per approved item via the existing `TaskService.CreateSubTask` path, with `ParentTaskID` set, plus `SequenceIndex` (0-based) and `DependsOn` (sibling IDs, if the split identified inter-child dependencies)
- The parent task transitions to `coding`-equivalent running state and begins dispatching children one at a time in `SequenceIndex` order — v1 execution is strictly sequential regardless of `DependsOn` (see Non-goals in proposal.md)
- Children execute on the parent's own workspace/branch lineage (continuous, shared worktree), not an isolated worktree per child

## Scenario: A child task attempt runs
**When:**
- A child task (or any task) begins CLI execution

**Then:**
- A new `TaskAttempt` row is created, referencing the `Task` and recording start time, sandbox/container reference, and (on completion) exit status, tokens in/out, cost, duration
- A retry of the same child creates a new `TaskAttempt`, never mutates or deletes a prior one — the `Task` row's status reflects the latest attempt's outcome, but attempt history is durable

## Scenario: A child task completes successfully
**When:**
- Child task's latest `TaskAttempt` finishes with exit 0 and passes its own verification (tests/build, same as any normal task today)
- The child's actual changed files satisfy its Contract's `output_expected` (or a deviation is recorded, not silently dropped)

**Then:**
- Child's outcome is recorded as a structured `ChildTaskSummary` (changed files, test summary, cost/duration, one-line outcome)
- The next child starts with that `ChildTaskSummary` available as `input.previous_summary` in its own Contract — never the prior child's raw transcript

## Scenario: All children complete
**When:**
- Every child task reaches a terminal success state

**Then:**
- The Reduce step (deterministic code, not an LLM call) aggregates child `ChildTaskSummary` records into the parent's final result: union of changed files, aggregated test pass/fail counts, summed cost/duration
- Parent task transitions to `completed`/`merged`-equivalent per the existing lifecycle
- An LLM may run *after* Reduce, off the critical path, to turn the structured aggregate into prose (e.g. one combined PR description) — it never participates in computing the aggregate itself

## Failure Scenario: A child task fails
**When:**
- A child task's latest `TaskAttempt` fails (non-zero exit, timeout, or its own retry budget exhausted)

**Then:**
- The parent task transitions to `blocked` (not `failed`) with the failing child's ID and `SequenceIndex` recorded
- Already-completed sibling children's changes are NOT rolled back — they represent real, verified progress on the shared workspace lineage
- The parent is resumable: retrying it creates a new `TaskAttempt` for the failed child only (children after it remain unstarted; children before it are untouched) and continues sequential dispatch from there
- `blocked` only ever originates from a child failure under this feature; a task with no children still uses `failed` exactly as it does today

## Failure Scenario: Analyze proposes a split the operator rejects (manual mode)
**When:**
- Operator declines the proposed decomposition under `decomposition_mode=manual`

**Then:**
- Task proceeds as a normal, single, non-decomposed task (today's existing path)

## Failure Scenario: A child's actual output doesn't match its Contract
**When:**
- A child completes (exit 0) but its changed-file set doesn't include files listed in its Contract's `output_expected`

**Then:**
- The mismatch is recorded on the `ChildTaskSummary` as a warning, surfaced in the UI at that child's position in the sub-task tree — it does not block the next child from starting, but it is visible before Reduce runs, not buried in a later child's confusing failure

## Invariants
- A child task's `ParentTaskID` is immutable once set (unchanged from today).
- A parent task with children never runs its own CLI step directly once children exist — execution always happens at the child level; the parent is a coordination/aggregation record only.
- No single child task's `output_expected` is planned to approach the provider's per-turn output-token ceiling — if analyze cannot propose a split under that ceiling, it must say so explicitly rather than silently producing an oversized child.
- Every `TaskAttempt` belongs to exactly one `Task`; a `Task` may have zero (not yet run) or many (retried) attempts.
- `blocked` is reachable only from a running decomposed parent with a failed child; it is not a status a non-decomposed task can enter.

## Rules
- Child tasks execute strictly in `SequenceIndex` order (v1 is sequential; `DependsOn` is stored but not used for scheduling).
- The Reduce aggregation step never re-sends a child's full raw CLI transcript as input context to a later step — only its structured `ChildTaskSummary`.
- `decomposition_mode` defaults to `manual` for any project/task that hasn't explicitly set it.

## Constraints
- No change to the existing single-task path's behavior, timeouts, or checkpointing — decomposition is strictly additive and opt-in via the Complexity Score threshold plus `decomposition_mode`.
- `CreateSubTask`/`ListSubTasks` (already shipped) remain callable directly for manual sub-task creation outside this feature's automatic split flow — this feature does not remove that capability.
