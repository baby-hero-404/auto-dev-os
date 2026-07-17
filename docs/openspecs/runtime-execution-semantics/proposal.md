# Proposal: Runtime Execution Semantics Hardening

## Why

Evidence from task `9e19c625` (tool-zentao, 4 backend subtasks) reveals a systemic pattern: the orchestrator's patch-retry loop allows partially-completed or empty work to propagate as "success", and the LLM wastes the majority of its iteration budget on re-discovery rather than implementation.

### Hard Evidence (from real execution logs)

| Metric | Value |
|--------|-------|
| Total LLM calls | 148 |
| Total tokens consumed | 2,116,919 |
| TELEMETRY-VIOLATION events | 55 |
| Budget exhaustions (salvage) | 12 |
| Empty checkpoint commits | 4 |
| Git directory failures | 16 |
| Re-discovery file reads | 19 unique paths read 2-9× across retries |
| Step failures | 4 (all eventually marked "success") |
| Expected deliverables | 4 files |
| Actually delivered | 1 file (25% completion rate) |

### Root Cause Chain

```
1. Agent starts coding step → spends 50-75% of tool budget on file discovery
2. Runs out of iteration budget before completing implementation
3. Salvage logic captures partial edit (often just a package declaration)
4. Tests pass trivially (no test targets, or the stub compiles)
5. patchRetryLoop marks the step as SUCCESS
6. Next step inherits an incomplete workspace
7. Next agent re-discovers the same files, wastes budget again
8. Cycle repeats across all 4 subtasks
```

### Delivery Outcome

Out of 4 planned subtasks (sqlite repository, zentao client, sync engine, CLI entrypoint), only **1 file** (`internal/zentao/client.go`, 334 lines) was successfully delivered. 3 critical files are completely missing despite every step showing `success` status.

---

## What Changes

### Issue 1: Salvage-as-Success Loophole (FIXED — this session)
- **[DONE]** Partial salvage with passing tests now forces retry instead of marking success
- **[DONE]** Missing summary response now triggers retry instead of silent pass-through
- **[DONE]** Missing diff/patch in non-agentic mode now triggers retry

### Issue 2: Agent Discovery Waste (47% of all tool calls)
- Coding agents spend 31-75% of their tool calls on read_file/list_files/file_exists
- `code_backend_1` spent 75% of its budget just reading files
- `code_backend_2` read `internal/zentao/client.go` **9 times** across retries
- `go.mod` read **12 times** total across all steps

### Issue 3: Prompt Inflation (235% growth)
- `code_backend_2` prompt grew from 8,716 to 29,238 tokens (235% increase)
- Conversation context accumulates tool results without compression
- Each retry rebuilds messages from scratch but carries forward uncompressed history

### Issue 4: State Machine Violations Indicate Phase Confusion
- 55 telemetry violations show agents using wrong tools for wrong phases
- Agent writes files during DISCOVERY phase (should only read)
- Agent reads files during FAILED/SALVAGED state (should stop)
- Shadow state machine detects but does not block — violations are advisory only

### Issue 5: Git Worktree Corruption Under Salvage
- `code_backend_3` attempt #1 encountered "fatal: not in a git directory" **16 times**
- Salvage checkpoint creation failed, leading to lost work
- Worktree had to be completely re-created on attempt #2

### Issue 6: Workspace Lock Contention
- Log: `workspace clone failed: workspace is locked in DB by another active process`
- Occurs when retry attempt overlaps with agent assignment for a different task
- Advisory lock prevents concurrent access but produces confusing errors

---

## Capabilities

### New Capabilities
- **Pre-hydrated Context**: Runtime injects file contents the agent needs based on FrozenContext, eliminating redundant discovery
- **Hard Tool Gating**: Shadow state machine enforcement becomes blocking, not advisory
- **Completion Quality Gate**: Step success requires both structural (summary, diff) AND semantic (acceptance criteria met) validation
- **Negative Memory**: Track failed tool calls and unsuccessful file reads to prevent repeated waste
- **Discovery Budget Cap**: Hard limit on discovery-phase tool calls (e.g., max 3 file reads before implementation must begin)

### Modified Capabilities
- **Salvage Logic**: Partial result always forces retry (already done)
- **Checkpoint Validation**: Git checkpoint must stage > 0 files to count as progress
- **Prompt Construction**: Inject pre-read file contents instead of letting agent discover them
- **State Machine**: Transition from advisory telemetry to blocking enforcement

### Removed Capabilities
- (none)

---

## Impact

| Area | Files Affected |
|------|----------------|
| Patch Retry Loop | `server/internal/orchestrator/steps/patch_retry_loop.go` |
| LLM Runner | `server/internal/orchestrator/llmrunner/runner.go`, `statemachineloop.go` |
| Coding Steps | `steps/code_backend.go`, `steps/code_frontend.go`, `steps/fix.go` |
| Worker | `server/internal/orchestrator/worker.go` |
| Prompt Builder | `server/internal/prompts/` |
| Workspace Manager | `server/internal/orchestrator/repoutil/worktrees.go` |
| Checkpoint Logic | `server/internal/orchestrator/repoutil/checkpoints.go` |
| Frontend Log Console | `web/src/components/dashboard/log-console.tsx` |
