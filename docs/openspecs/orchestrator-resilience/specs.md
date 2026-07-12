# Specs: Orchestrator Resilience

## Modified Requirements

### REQ-M01: Resilient Subtask Routing
> ❌ Status: Not Started

**Scenario:**
- WHEN the LLM generates a TasksMD heading that `workflow.ClassifyHeading` buckets as `frontend` via a signal `extractSpecsSectionForSubtask`'s own keyword list doesn't recognize (e.g. "page", "component", "style", "layout", "model", "service", "handler")
- THEN `extractSpecsSectionForSubtask` must classify the heading identically to how `ParseTasksMD` bucketed it into `subtasks["backend"]`/`["frontend"]`, so the Nth-heading index used to slice `SpecsMD` never drifts from the index used to slice `TasksMD`.
- Concretely: `extractSpecsSectionForSubtask` must call the same `workflow.ClassifyHeading` function `ParseTasksMD` uses, not a separately maintained keyword list.

### REQ-M02: Guaranteed Execution Manifest
> ❌ Status: Not Started

**Scenario:**
- WHEN the prompt tokens exceed the configured budget
- THEN the `optimizeBudget` function must drop mutable context (like semantic snippets) based on priority
- AND the `Execution Manifest` section must never be dropped.

### REQ-M03: Safe Plan Output Parsing
> ❌ Status: Not Started

**Scenario:**
- WHEN the Plan step outputs an unexpected JSON type for the subtasks array
- THEN the coding step must fail gracefully (logged warning, no `tasks_md` update) instead of a runtime panic. (Note: the top-level `recover()` in `orchestrator/worker.go` already stops this from taking down the whole process, but the task still fails ungracefully today and the panic is opaque in logs — this requirement replaces that with a clean, logged skip.)

### REQ-M04: Sandbox Commit Data Preservation
> ❌ Status: Not Started

**Scenario:**
- WHEN `runPatchRetryLoop` succeeds but the subsequent `commitSandbox` fails
- THEN the failed coding step records no commit checkpoint, and a later job resume calls `RestoreGitCheckpoint` against the prior successful checkpoint
- AND `RestoreGitCheckpoint` must snapshot any dirty worktree state to a recoverable branch before performing its destructive `checkout`/`reset --hard`/`clean -fd`, so the uncommitted changes are preserved in git history rather than permanently discarded.
- AND `CommitRoleWorktrees` should retry the commit itself a bounded number of times before surfacing an error, since the underlying failure is typically transient (lock/IO contention).

### REQ-M05: Accurate Fix-Step Context
> ❌ Status: Not Started
> (Corrected scope: the Review step already satisfies this — see `review.go:229-243` — this requirement now targets the Fix step only.)

**Scenario:**
- WHEN the Fix step is assembling its instruction
- THEN it must load `FrozenContext` (mirroring `review.go`'s pattern) and inject `AcceptanceCriteria` and `ExecutionBoundaries`, instead of relying solely on the PR-rejection feedback / review findings JSON it uses today.
- AND `builder.go`'s Execution Manifest construction for coding steps must include `acceptance_criteria`/`execution_boundaries` when `stepID == workflow.StepFix`, since Fix (unlike `code_backend`/`code_frontend`) has no subtask index and therefore never receives these via `extractSpecsSectionForSubtask` either.
