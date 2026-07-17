# Specs: Runtime Execution Semantics Hardening

## Added Requirements

### REQ-001: Salvage Must Force Retry
> ✅ Status: Implemented

**Scenario:**
- WHEN the agentic tool loop exhausts its iteration budget
- AND 1+ edits were applied (partial result)
- AND targeted tests pass trivially (no test targets or stub compiles)
- THEN the system MUST retry with a continuation prompt
- AND MUST NOT mark the step as success until max retries exhausted
- AND log message "Tool loop exhausted iterations" MUST appear

**Evidence:** `code_backend_0` through `code_backend_2` all produced salvage commits with `STAGED_COUNT=0` empty checkpoints yet were marked `success`.

---

### REQ-002: Missing Summary Must Force Retry
> ✅ Status: Implemented

**Scenario:**
- WHEN the LLM completes an agentic coding step
- AND the response JSON does not contain a non-empty `summary` string
- THEN the system MUST retry with instruction to provide summary
- AND MUST NOT mark the step as success

**Evidence:** Several iterations returned tool calls only with no final summary, yet the step completed successfully.

---

### REQ-003: Missing Diff/Patch Must Force Retry (Non-Agentic)
> ✅ Status: Implemented

**Scenario:**
- WHEN the coding step uses non-agentic (diff-based) mode
- AND the LLM response contains no extractable patch
- THEN the system MUST retry
- AND MUST NOT mark the step as success

---

### REQ-004: Empty Checkpoint Must Not Count as Progress
> ❌ Status: Not Started

**Scenario:**
- WHEN `CreateGitCheckpoint` runs
- AND `STAGED_COUNT=0` (no files staged)
- THEN the checkpoint MUST be recorded as `empty` in the database
- AND downstream logic (resume, restore) MUST treat it as non-existent
- AND the step MUST NOT be considered "checkpointed successfully"

**Evidence:** 4 empty checkpoint commits were created. Log: `"checkpoint code_backend_2 for repo ... staged 0 files — commit is empty, nothing from this step was captured"`, yet step still marked success.

---

### REQ-005: Pre-Hydrated Context Injection
> ❌ Status: Not Started

**Scenario:**
- WHEN a coding step begins execution
- AND FrozenContext contains a list of affected files with content
- THEN the prompt MUST include pre-read file contents
- AND the agent SHOULD NOT need to call `read_file` for already-known files
- AND discovery tool calls MUST decrease by ≥40% compared to baseline

**Evidence:** `code_backend_2` read `internal/zentao/client.go` 9× and `go.mod` 5× across retries. `code_backend_1` spent 75% of budget on discovery.

---

### REQ-006: Discovery Budget Cap
> ❌ Status: Not Started

**Scenario:**
- WHEN a coding agent is in DISCOVERY phase
- AND it has made N consecutive read-only tool calls (N configurable, default 5)
- THEN the runtime MUST inject a nudge: "Discovery budget reached. Begin implementation."
- AND subsequent read-only calls beyond N+2 MUST be rejected with an error message

**Evidence:** `code_backend_2` made 34/59 (58%) discovery calls. `code_backend_1` made 6/8 (75%) discovery calls.

---

### REQ-007: Hard Tool Gating by State Machine Phase
> ❌ Status: Not Started

**Scenario:**
- WHEN the shadow state machine reports a TELEMETRY-VIOLATION
- AND the phase is FAILED or SALVAGED
- THEN the tool call MUST be rejected (return error to LLM)
- AND the rejection MUST include the reason and current phase

- WHEN the phase is DISCOVERY
- AND the tool is a write tool (create_file, search_replace)
- THEN the state machine MUST transition to IMPLEMENTATION (current behavior)
- AND log a warning (current behavior)

**Evidence:** 55 TELEMETRY-VIOLATION events in a single task. Agent continued calling tools after entering SALVAGED state.

---

### REQ-008: Negative Memory for Failed Tool Calls
> ❌ Status: Not Started

**Scenario:**
- WHEN a tool call fails (e.g., file not found, permission error)
- THEN the runtime MUST record the failed path/action in a per-step negative memory
- AND subsequent retry attempts MUST include a "do not repeat" list in the prompt
- AND the agent MUST NOT re-attempt the same failed action

**Evidence:** Agent repeatedly tried `file_exists`, `list_files` on the same paths after failures.

---

### REQ-009: Prompt Reasoning Compression Between Retries
> ❌ Status: Not Started

**Scenario:**
- WHEN a retry attempt begins (attempt ≥ 2)
- THEN the prompt MUST include a compressed summary of previous attempt results
- AND MUST NOT include the full conversation history of the previous attempt
- AND prompt size for retry attempt MUST NOT exceed 120% of the base prompt size

**Evidence:** `code_backend_2` prompt grew from 8,716 to 29,238 tokens (235% growth).

---

### REQ-010: Git Worktree Resilience During Salvage
> ⚠️ Status: Partially Addressed

**Scenario:**
- WHEN `CreateGitCheckpoint` is called during salvage
- AND the worktree path is invalid or corrupted
- THEN the checkpoint MUST fail gracefully (no panic)
- AND the system MUST attempt to re-create the worktree on the next retry
- AND log a clear error identifying the worktree path issue

**Evidence:** `code_backend_3` attempt #1: "fatal: not in a git directory" 16 times. Worktree re-created on attempt #2.

---

## Modified Requirements

### REQ-M01: Log Console Group Handling for Paused Steps
> ✅ Status: Implemented

**Scenario:**
- WHEN a step emits `paused` status
- THEN the log console MUST keep the group open
- AND continue collecting logs until `success` or `failed` is received
- AND MUST NOT permanently display `PAUSED` status after resume

---

### REQ-M02: Log Console Step Name Formatting
> ✅ Status: Implemented

**Scenario:**
- WHEN a log group displays step name `code_backend_N`
- THEN the UI MUST display `Backend Execution N+1` (1-based)
- AND the naming MUST match the Workflow Timeline panel

---

## Removed Requirements
- (none)
