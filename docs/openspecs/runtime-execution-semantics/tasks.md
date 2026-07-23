# Tasks: Runtime Execution Semantics Hardening

## Phase 1: Salvage & UI Hardening ✅ DONE

- [x] **T-001**: Force retry on partial salvage when tests pass trivially
  - File: `steps/patch_retry_loop.go` L188-197
  - REQ: REQ-001

- [x] **T-002**: Force retry when LLM response missing summary (agentic mode)
  - File: `steps/patch_retry_loop.go` L239-248
  - REQ: REQ-002

- [x] **T-003**: Force retry when LLM response missing diff/patch (non-agentic mode)
  - File: `steps/patch_retry_loop.go` L336-345
  - REQ: REQ-003

- [x] **T-004**: Fix log console paused group not closing properly
  - File: `web/src/components/dashboard/log-console.tsx` L37-48
  - REQ: REQ-M01

- [x] **T-005**: Fix log console 0-based step name mismatch
  - File: `web/src/components/dashboard/log-console.tsx` L373-385
  - REQ: REQ-M02

---

## Phase 2: Checkpoint & Context Optimization

- [x] **T-006**: Empty checkpoint validation
  - File: `server/internal/orchestrator/repoutil/checkpoints.go`
  - Change: Return `CheckpointResult` struct with `IsEmpty` flag
  - Update callers in `worker.go` and `patch_retry_loop.go` to check `IsEmpty`
  - REQ: REQ-004
  - Priority: P0
  - Estimated effort: S

- [x] **T-007**: Pre-hydrated context injection for coding steps
  - File: `steps/code_backend.go`, `steps/code_frontend.go`
  - Change: Read FrozenContext affected files, inject contents into prompt
  - Cap at 4,000 tokens with truncation
  - REQ: REQ-005
  - Priority: P0
  - Estimated effort: M

- [x] **T-008**: Discovery budget cap in agentic tool loop
  - File: `server/internal/orchestrator/llmrunner/runner.go`
  - Change: Add `discoveryTracker` that counts consecutive read-only calls
  - Inject nudge at threshold, block at threshold+2
  - REQ: REQ-006
  - Priority: P0
  - Estimated effort: M

- [x] **T-009**: Update unit tests for T-006/T-007/T-008
  - File: `steps/patch_retry_loop_test.go`, `steps/code_step_test.go`, `llmrunner/runner_test.go`
  - Priority: P0
  - Estimated effort: M

---

## Phase 3: Runtime Intelligence ✅ DONE

- [x] **T-010**: Hard tool gating for FAILED/SALVAGED states
  - File: `server/internal/orchestrator/llmrunner/runner.go` L350-353
  - Change: Return error string to LLM instead of just logging warning
  - REQ: REQ-007
  - Priority: P1
  - Estimated effort: S

- [x] **T-011**: Negative memory tracking
  - File: `llmrunner/toolloop.go` (FailedCalls in ToolLoopResult), `steps/patch_retry_loop.go` (cumulative injection)
  - Change: Record failed tool calls; render "do not repeat" list on retry
  - REQ: REQ-008
  - Priority: P1
  - Estimated effort: M

- [x] **T-012**: Prompt reasoning compression between retries
  - File: `steps/patch_retry_loop.go` (`compressErrorText` function)
  - Change: Compress error text >100 lines to head+tail with truncation marker
  - REQ: REQ-009
  - Priority: P1
  - Estimated effort: M

- [x] **T-013**: Git worktree resilience during salvage
  - File: `server/internal/orchestrator/repoutil/worktrees.go` (CreateGitCheckpoint)
  - Change: Validate worktree path before git operations; auto-recreate if invalid
  - REQ: REQ-010
  - Priority: P1
  - Estimated effort: S

- [x] **T-014**: Update unit tests for T-010/T-011/T-012/T-013
  - Files: `llmrunner/toolloop_failedcalls_test.go`, `steps/phase3_test.go`, `repoutil/worktrees_resilience_test.go`
  - Tests: 11 new tests, all passing
  - Priority: P1
  - Estimated effort: M

---

## Validation Criteria

After all phases are complete, re-run the same task type (4-subtask Go backend) and verify:

| Metric | Baseline (9e19c625) | Target |
|--------|---------------------|--------|
| Discovery tool calls | 47% of total | ≤ 20% |
| Prompt inflation | 235% growth | ≤ 50% |
| TELEMETRY violations | 55 | ≤ 10 |
| Salvage-as-success | 4 occurrences | 0 |
| Empty checkpoints passed | 4 | 0 |
| Delivery completion rate | 25% (1/4 files) | ≥ 75% |
| Total token consumption | 2.1M | ≤ 1.2M |

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
