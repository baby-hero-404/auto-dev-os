# Tasks: Orchestrator Resilience

## P0 — Critical

### Task 1.1: Fix Plan Output Type Assertion
> Links to: REQ-M03

**Acceptance Criteria:**
- [x] In `server/internal/orchestrator/steps/code_backend.go` and `code_frontend.go`, the line parsing the task text uses `if taskText, ok := beTasks[taskIdx].(string); ok` instead of a direct cast.
- [x] Appropriate warning is logged if the type assertion fails.

### Task 1.2: Mark Execution Manifest as Immutable
> Links to: REQ-M02

**Acceptance Criteria:**
- [x] In `server/internal/prompts/builder.go`, the `Execution Manifest` section is instantiated with `IsImmutable: true` so it is not dropped by `optimizeBudget`.

## P1 — High

### Task 2.1: Fix Subtask Routing Logic
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] Export `classifyHeading` as `workflow.ClassifyHeading` in `server/internal/workflow/parser.go`.
- [x] In `server/internal/prompts/helpers.go`, `extractSpecsSectionForSubtask` calls `workflow.ClassifyHeading` instead of its own drifted `isRole` keyword list.
- [x] Add unit tests for `extractSpecsSectionForSubtask` covering headings that only match via signals `isRole`'s old list omitted (e.g. "page", "component", "handler").

### Task 2.2: Add FrozenContext to Fix Step
> Links to: REQ-M05
> (Corrected from "Ensure Review Uses FrozenContext" — Review already does this; Fix does not.)

**Acceptance Criteria:**
- [x] In `server/internal/orchestrator/steps/fix.go`, the instruction is built with `LoadFrozenContext` and injects `AcceptanceCriteria`/`ExecutionBoundaries`, mirroring `review.go:229-243`.
- [x] In `server/internal/prompts/builder.go`, the `isCodingStep` Execution Manifest branch includes `acceptance_criteria`/`execution_boundaries` when `stepID == workflow.StepFix`.

## P2 — Medium

### Task 3.1: Mitigate Sandbox Commit Failures
> Links to: REQ-M04

**Acceptance Criteria:**
- [x] In `server/internal/orchestrator/repoutil/worktrees.go`, `RestoreGitCheckpoint` snapshots any dirty worktree to a `rescue/*` branch before its destructive `checkout`/`reset --hard`/`clean -fd`.
- [x] `CommitRoleWorktrees` retries the commit script a bounded number of times (with backoff) before returning an error.

## Verification

- `go build ./...`, `go vet ./...`, and `go test ./...` all pass after these changes.
- New test: `TestExtractSpecsSectionForSubtask_ClassificationMatchesParseTasksMD` in `server/internal/prompts/helpers_test.go` reproduces the pre-fix drift and confirms the fix.
- No test coverage exists for `server/internal/orchestrator/repoutil` (pre-existing gap, not introduced by this change) — the rescue-branch and retry logic were verified by build/vet/read-through only, not by an integration test against a real sandbox.

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
