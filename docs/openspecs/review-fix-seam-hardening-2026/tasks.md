# Tasks: Review→Fix Seam Hardening 2026

> Dependency order: 1.1 → 1.3 (role fix must land before persona reuse); 1.2 → 2.1 (diff format
> must be repo-relative before the canonicalizer's contract is meaningful); 2.2 depends on 1.1
> (persona selection consumes `effectiveRoleForStep`). Every task cites the report evidence it
> closes: `docs/reports/task-8291a25e-nested-path-trace-verification-report.md`.

## Implementation Protocol

- **Execution mode: one task per commit, in dependency order** (1.1 → 1.2 → 1.3 → 2.1 → 2.2 → 3.1).
  Do NOT batch P0+P1 into one change: Task 1.2 changes a wire format (diff text) consumed by
  multiple components, and its consumer-audit gate must be verifiable in isolation. Each commit
  message references its Task ID and REQ IDs.
- **Test runner:** backend suite is `make test-be` (= `cd server && go test ./... -v -count=1`).
  During development, target the affected packages:
  `cd server && go test ./internal/orchestrator/... ./internal/tool/... ./pkg/paths/... -count=1`.
  No frontend (`test-fe`/Playwright) impact expected; run `make test-be` green before each commit.
- **Working-tree hotfixes:** Tasks 1.1 and 1.2 formalize uncommitted hotfixes currently live in
  the working tree (`llm_step.go:87` remap, `capability.go` CapCreate, `getDiffPrefixes` bypass).
  Each task supersedes its hotfix — the task's diff replaces, not stacks on, the hotfix.
- **Role mapping authority:** the full step×role matrix lives in design.md § Role Resolution
  Matrix. Summary: the remap is *capability-driven*, not name-based — any edit-expected step
  (`fix`, `code_backend*`, `code_frontend*`) arriving under a role lacking `CapEdit`+`CapCreate`
  resolves to `coderRoleForTask(task)` (`frontend` iff `PrimaryCategory ∈ {frontend, ui, ux}`,
  else `backend`). Read-only steps are never remapped. This covers more than `fix`+`reviewer`
  because agent assignment is by task status with fallback chains (`agent_manager.go:67`), so
  even `code_backend` can arrive under a Planner agent.
- **Authorization rejections are loop-correctable, not fail-fast** (design.md § Authorization
  Rejection Semantics): the model receives the allowed-tool list and a do-not-retry statement;
  existing circuit breaker + budget terminate pathological repetition. No new fail-fast path.

## P0 — Critical (closes the observed failure end-to-end)

### Task 1.1: Unify role resolution for advertisement + enforcement
> ✅ Status: Completed
> Links to: REQ-001, REQ-M02, REQ-006 · Closes report findings N1, N2

**Acceptance Criteria:**
- [x] Add `stepRequiresEditCaps(stepID)` + `effectiveRoleForStep(stepID, agentRole, task)` implementing the full matrix in design.md § Role Resolution Matrix (capability-driven via `tool.AllowedForRole`, coder role from `analysis.PrimaryCategory`).
- [x] Use the resolved role for BOTH `ToolsForRole` (`llm_step.go:48`) and `NewBoundaryCheckedToolExecutor` (`llm_step.go:92`); delete the executor-only remap at `llm_step.go:87-89` (supersedes the working-tree hotfix).
- [x] Commit the `CapCreate` addition for `backend`/`frontend` in `DefaultRoleProfiles` (`internal/tool/capability.go` — currently an uncommitted hotfix).
- [x] Make `Registry.Execute`'s authorization rejection actionable per design.md § Authorization Rejection Semantics (`internal/tool/registry.go:49`): in-loop tool error (NOT fail-fast), lists the role's available tools from `ToolsForRole`, states the rejection is permanent for this step; existing circuit breaker/budget unchanged.
- [x] Unit test (table-driven over the matrix): for each `stepID × agentRole` row — `fix`/`code_backend`/`code_frontend` under `reviewer`, `planner`, `qa`, `backend`, `frontend`, and an unknown role; plus `review`/`analyze` under `reviewer`/`planner` — advertised tool names == executable tool names, and read-only steps are never remapped.
- [x] Unit test: `PrimaryCategory: "frontend"` task remaps fix under reviewer to `frontend`; backend-category and empty-analysis tasks remap to `backend`.
- [x] Regression test (task 8291a25e): fix step under a reviewer agent advertises `search_replace` + `create_file`, and executing `create_file` through the boundary executor succeeds authorization.
- [x] Verify: `cd server && go test ./internal/orchestrator/... ./internal/tool/... -count=1` green, then `make test-be` green.

### Task 1.2: Repo-relative workspace diffs
> ✅ Status: Completed
> Links to: REQ-002 · Closes report Step-2 root cause + finding N4

**Acceptance Criteria:**
- [x] Remove `--src-prefix`/`--dst-prefix` injection from the `GetWorkspaceDiff` Python script (`gitops/client.go:195`) and any path-prefixing in `GetWorkspaceChangedFiles`.
- [x] Delete `getDiffPrefixes` and its interpolation in `GetDiff`/`GetPRDiff` (supersedes the partial working-tree hotfix; commit the resolution).
- [x] Audit every consumer of these diffs for prefix assumptions before landing: `ApplyPatch`, snapshot restore (`llm_step.go:121`, `patch_retry_loop.go:165`), artifact/diff persistence, and the web diff viewer.
- [x] Integration test: multi-repo workspace diff for a repo at `code/repos/x/main/` yields headers `a/cmd/main.go`, and `--- Repository: x` carries attribution.
- [x] Verify: `cd server && go test ./internal/orchestrator/gitops/... -count=1` green, then `make test-be` green.

### Task 1.3: Typed + canonicalized review→fix seam
> ✅ Status: Completed
> Links to: REQ-003, REQ-M01 · Closes report Step-3 root cause; first applied slice of execution-semantics-2026 typed contracts

**Acceptance Criteria:**
- [x] Add `models.ReviewFinding` (design.md § Data Models); replace `getReviewFindings(parsed) any` (`steps/review.go:38`) with `ParseReviewFindings` accepting today's `findings`/`array`/single-object shapes.
- [x] Add `paths.CanonicalizeRepoRelative` with table-driven tests covering: already-relative, single workspace prefix, doubled prefix (the call-131 case), foreign-repo prefix (rejected), traversal escape (rejected).
- [x] Apply canonicalization in `fix.go` before instruction rendering; drop + warn-log unresolvable findings.
- [x] Fix the instruction header (`fix.go:163`): paths are repository-relative to repository `<name>` (replaces "relative to your workspace root").
- [x] End-to-end regression test reproducing the incident: finding with `code/repos/tool_zentao/main/cmd/sync/main.go` → fix prompt contains `cmd/sync/main.go`; executing the resulting `create_file` lands at `<repo>/cmd/sync/main.go`, and `<repo>/code/repos/...` is never created.
- [x] Verify: `cd server && go test ./pkg/paths/... ./internal/orchestrator/steps/... -count=1` green, then `make test-be` green.

## P1 — High (defense in depth + persona correctness)

### Task 2.1: Tool-layer self-nesting guard
> ✅ Status: Completed
> Links to: REQ-004 · Closes report v4.0-F10/F17

**Acceptance Criteria:**
- [x] `SafeWorkspacePath` (`internal/tool/helpers.go`) rejects `code/repos/…`-prefixed relPaths when the workspace root is a repo checkout, with the actionable message from design.md.
- [x] `patch.EvaluatePolicy` classifies self-nested paths as `SeverityError` (loop-correctable), not Warning/auto-expansion.
- [x] Unit tests: guard triggers for `create_file`/`search_replace`; does not trigger for a legitimate repo that genuinely contains a `code/` source directory (`code/foo.go` must pass).

### Task 2.2: Coder persona for fix step
> ✅ Status: Completed
> Links to: REQ-005 · Depends on: 1.1 · Closes report finding N3

**Acceptance Criteria:**
- [x] Prompt assembly selects the persona from `effectiveRoleForStep` (same resolved role as advertisement/enforcement), so fix renders a coder role prompt — never `# Planner Role` / `# Reviewer Role`.
- [x] Assert in a prompt-assembly test that the fix system prompt contains no "Do NOT write implementation code" constraint.
- [x] Verify: `cd server && go test ./internal/prompts/... -count=1` green, then `make test-be` green.

## P2 — Verification

### Task 3.1: Incident replay verification
> ✅ Status: Completed
> Links to: REQ-001..006

**Acceptance Criteria:**
- [x] Re-run the zentao-auto task (or a reduced fixture reproducing review→fix with a prefixed-path finding) with all P0/P1 changes: workflow must complete fix without `ErrNoProgress`, with zero authorization rejections and zero nested directories in the workspace.
- [x] Grep assertion on the run's LLM traces: no prompt contains `code/repos/<repo>/<branch>/` file paths in findings or diff headers.

## P3 — Post-implementation code review fixes

> Uncovered by an independent code review of the Task 1.1–2.2 implementation, which found the
> single-repo (zentao-auto) scenario solid but several multi-repo blind spots the acceptance
> criteria above never exercised. All fixes verified with `go build ./...`, `go vet ./...`, and
> `go test ./... -count=1` green, plus targeted regression tests reproducing each bug before the
> fix and passing after.

### Task 4.1: Fix multi-repo diff-to-patch round-trip
> ✅ Status: Completed
> Links to: REQ-002

**Acceptance Criteria:**
- [x] `SplitPatchByRepoWithWorkspace` now splits on the `--- Repository: <name>` marker line (the only repo-attribution signal left once REQ-002 removed repo-name path prefixes) instead of inferring repo identity from per-file paths, which broke for any workspace with >1 repo (a bare subdirectory like `internal` was misread as the repo name, or the repo resolved to `""`, merging every repo's diff blocks into one bucket). Path-based inference is kept as the fallback for single-repo diffs (`GetDiff`/`GetPRDiff`) that never carry the marker.
- [x] Extracted shared prefix-stripping logic (`stripWorktreeAndBranchPrefix`, `mainBranchDirFor`) so both the legacy path-inference branch and the new header-based branch use identical worktree/branch-directory stripping rules — no behavior change to the existing single-repo test cases.
- [x] Regression test `TestRunner_SplitPatchByRepoWithWorkspace_MultiRepo` (applier_test.go): a 2-repo header-delimited patch splits into exactly 2 correctly-attributed, correctly-cleaned repo patches.
- [x] All pre-existing `NormalizePatchPath`/`SplitPatchByRepoWithWorkspace` tests still pass unchanged (behavior-preserving refactor for the single-repo/no-workspace-metadata paths).

### Task 4.2: Make EvaluatePolicy's self-nesting guard workspace-aware
> ✅ Status: Completed
> Links to: REQ-004

**Acceptance Criteria:**
- [x] `EvaluatePolicy` takes a new `isRepoRootWorkspace bool` parameter; the self-nested-path guard only fires when true. Without this, a multi-repo task's tool workspace (the flat `CodeRoot`, where `code/repos/<repo>/...` is the CORRECT relative path) had every edit rejected as a false-positive boundary violation.
- [x] Extracted `tool.IsRepoCheckoutWorkspace` (used by both `SafeWorkspacePath` and the new `EvaluatePolicy` parameter at its `boundary_tool_executor.go` call site) so both guards agree on what counts as a repo-root workspace, instead of `SafeWorkspacePath` computing it correctly and `EvaluatePolicy` not computing it at all.
- [x] `applier.go`'s call site passes `false` (its `repoRelPath` is always `"<repoName>/<file>"`, never `"code/repos/..."`, so the guard is inapplicable there regardless).
- [x] Regression test added to `TestEvaluatePolicy`: the same `code/repos/repoB/main/utils.go` path is `SeverityError` (nesting guard) when `isRepoRootWorkspace=true`, but does NOT trigger the nesting guard's reason text when `false`.

### Task 4.3: Restore requires_fix:true as an independent actionability signal
> ✅ Status: Completed
> Links to: REQ-003

**Acceptance Criteria:**
- [x] Added `RequiresFix bool` to `models.ReviewFinding`; `ParseReviewFindings` populates it from `fMap["requires_fix"]`; `hasActionableFindings` checks it as an independent OR condition (not derived from `Severity`) — restoring the pre-Task-1.3 behavior that a `requires_fix:true` finding with no/unrecognized severity string still routes to the fix step. The typed-contract rewrite had dropped this field entirely, silently routing such findings straight to Testing.
- [x] Regression test `TestReviewStep_RequiresFixTrue` (review_test.go): a finding with `requires_fix:true` and no severity transitions the task to `TaskStatusFixing`.
- [x] Unit test `TestParseReviewFindings_FieldMapping` locks in the full field mapping (Repo, File, Line, Severity, Recommendation, RequiresFix) directly, closing the gap where only the JSON→struct conversion had no dedicated test.

### Task 4.4: Consolidate duplicated role-resolution logic
> ✅ Status: Completed
> Links to: REQ-001, REQ-005

**Acceptance Criteria:**
- [x] `stepRequiresEditCaps`/`effectiveRoleForStep`/`coderRoleForTask` existed as two independent copies (`orchestrator/llm_step.go` and `prompts/builder.go`) because the two packages can't import each other. Moved the single implementation to `internal/tool/rolepolicy.go` (exported `StepRequiresEditCaps`/`EffectiveRoleForStep`/`CoderRoleForTask`), which both `orchestrator` and `prompts` already import without creating a cycle.
- [x] Both call sites (`llm_step.go`'s tool advertisement/executor wiring, `builder.go`'s persona selection) now call the shared `tool.` functions; no local copies remain.
- [x] `TestStepRequiresEditCaps`/`TestEffectiveRoleForStep` moved to `internal/tool/rolepolicy_test.go` alongside the implementation; `TestRegression_8291a25e` (orchestrator package, exercises `steps.NewBoundaryCheckedToolExecutor`) updated to call `tool.EffectiveRoleForStep`.

### Task 4.5: Fix double "Error: Error:" prefix in tool rejection messages
> ✅ Status: Completed
> Links to: REQ-006

**Acceptance Criteria:**
- [x] `NewRegistryToolExecutor` (tool_executor.go) no longer prepends a second `"Error: "` when `res.Message` (e.g. `Registry.Execute`'s authorization rejection) already starts with one — this was the literal `"Error: Error: role \"reviewer\" is not authorized..."` artifact quoted in the task 8291a25e trace report.
- [x] Regression test `TestNewRegistryToolExecutor_NoDoubleErrorPrefix`.

### Task 4.6 (informational, no code change): fix.go's hardcoded "main" branch parameter is correct
> ✅ Status: Verified, not a defect
> Links to: REQ-003

The review raised whether hardcoding `"main"` in `fix.go`'s two `paths.CanonicalizeRepoRelative(f.File, repoName, "main")` calls (rather than deriving it from `repo.DefaultBranch`/`repo.Paths.Main` the way `path_normalizer.go` does) could break repos whose default branch isn't `main`. Verified against `paths.OSWorkspacePaths.RepoMain` (`pkg/paths/workspace.go`): the fix step's tool workspace is always resolved via `RepoMain`, whose on-disk directory is literally named `"main"` regardless of the repo's git default branch — `path_normalizer.go`'s branch-derivation logic handles a different, more variable input (arbitrary LLM/external diff text), not this call site's own physical layout. Left as-is with a clarifying comment added in `fix.go` referencing `resolveAgenticWorkspace`.
