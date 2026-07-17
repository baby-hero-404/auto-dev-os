# Design: Runtime Execution Semantics Hardening

## Architecture Overview

The changes span three layers: **Orchestration Logic** (patch retry loop, worker), **Prompt Construction** (context injection), and **Runtime Intelligence** (tool gating, negative memory, budget caps).

```
┌─────────────────────────────────────────────────────────────┐
│                     Worker (worker.go)                       │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │ Step Runner   │  │ Checkpoint   │  │ State Machine     │  │
│  │              │  │ Validator    │  │ (BLOCKING mode)   │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬────────────┘  │
│         │                 │                  │               │
│  ┌──────▼─────────────────▼──────────────────▼────────────┐  │
│  │              patchRetryLoop (HARDENED)                  │  │
│  │  ┌───────────────┐  ┌────────────┐  ┌────────────────┐ │  │
│  │  │ Salvage Gate  │  │ Summary    │  │ Discovery      │ │  │
│  │  │ (force retry) │  │ Gate       │  │ Budget Cap     │ │  │
│  │  └───────────────┘  └────────────┘  └────────────────┘ │  │
│  └────────────────────────┬───────────────────────────────┘  │
│                           │                                  │
│  ┌────────────────────────▼───────────────────────────────┐  │
│  │           Prompt Compiler (NEW)                         │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │  │
│  │  │ Pre-Hydrated │  │ Negative     │  │ Compressed   │  │  │
│  │  │ Context      │  │ Memory       │  │ History      │  │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  │  │
│  └─────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Component 1: Checkpoint Validator

**File:** `server/internal/orchestrator/repoutil/checkpoints.go`

### Data Model Change

```go
type CheckpointResult struct {
    Hash       string
    StagedCount int
    IsEmpty    bool   // NEW: true when StagedCount == 0
}
```

### Behavior

- `CreateGitCheckpoint` returns `CheckpointResult` instead of just `string`
- Callers check `IsEmpty` before treating checkpoint as valid progress
- Empty checkpoints are stored in DB with `state.empty = true`
- Resume logic (`restoreTaskState`) skips empty checkpoints

---

## Component 2: Pre-Hydrated Context Injection

**File:** `server/internal/orchestrator/steps/code_backend.go`, `code_frontend.go`

### Design

Before calling `runPatchRetryLoop`, the coding step:

1. Reads `FrozenContext.AffectedFiles` from the plan step output
2. For each file that exists in the worktree, reads its content
3. Injects file contents into the prompt as a `## Pre-Read Files` section

```go
func buildPreHydratedContext(ctx context.Context, workspace WorkspaceLoader, task *models.Task, frozenCtx *FrozenContext, worktreeSuffix string) string {
    var sb strings.Builder
    sb.WriteString("\n\n## Pre-Read Files (do NOT re-read these)\n")
    for _, af := range frozenCtx.AffectedFiles {
        content, err := workspace.ReadFile(ctx, task, af.File, worktreeSuffix)
        if err != nil || content == "" {
            continue
        }
        sb.WriteString(fmt.Sprintf("\n### %s\n```\n%s\n```\n", af.File, content))
    }
    return sb.String()
}
```

### Token Budget

- Cap total pre-hydrated content at 4,000 tokens
- Prioritize files by relevance (affected files first, then dependencies)
- Truncate large files to first 200 lines

---

## Component 3: Discovery Budget Cap

**File:** `server/internal/orchestrator/llmrunner/runner.go`

### Design

Track consecutive read-only tool calls within the agentic loop:

```go
type discoveryTracker struct {
    consecutiveReads int
    maxReads         int  // default 5
    nudgeSent        bool
}

func (d *discoveryTracker) onToolCall(toolName string) (inject string, block bool) {
    if isReadOnlyTool(toolName) {
        d.consecutiveReads++
        if d.consecutiveReads >= d.maxReads && !d.nudgeSent {
            d.nudgeSent = true
            return "SYSTEM: Discovery budget reached. You have read enough files. Begin implementation now.", false
        }
        if d.consecutiveReads >= d.maxReads+2 {
            return "", true  // block the call
        }
    } else {
        d.consecutiveReads = 0  // reset on non-read tool
    }
    return "", false
}
```

---

## Component 4: Hard Tool Gating

**File:** `server/internal/orchestrator/llmrunner/runner.go`, `statemachineloop.go`

### Current Behavior (Advisory)

```go
if checkErr := shadowSM.CheckTool(name); checkErr != nil {
    r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("[TELEMETRY-VIOLATION] ..."))
    // tool still executes
}
```

### Target Behavior (Blocking for FAILED/SALVAGED)

```go
if checkErr := shadowSM.CheckTool(name); checkErr != nil {
    r.log(ctx, task.ID, nil, "warn", fmt.Sprintf("[TELEMETRY-VIOLATION] ..."))
    if shadowSM.Current() == "FAILED" || shadowSM.Current() == "SALVAGED" {
        return fmt.Sprintf("Tool %s is blocked: execution is in %s state. Stop and provide your summary.", name, shadowSM.Current()), nil
    }
}
```

---

## Component 5: Negative Memory

**File:** `server/internal/orchestrator/llmrunner/runner.go` (new struct)

### Data Model

```go
type NegativeMemory struct {
    FailedPaths    []string  // files that returned errors
    FailedTools    []string  // tool calls that failed
    mu             sync.Mutex
}

func (nm *NegativeMemory) Record(toolName, path, errorMsg string) { ... }
func (nm *NegativeMemory) Render() string { ... }  // for prompt injection
```

### Integration

- Created per-step in `patchRetryConfig`
- On each failed tool call result, `Record()` is called
- On retry, `Render()` output is appended to `retryErrorMsg`

---

## Component 6: Prompt Reasoning Compression

**File:** `server/internal/orchestrator/steps/patch_retry_loop.go`

### Design

On retry (attempt ≥ 2), instead of carrying full conversation history:

```go
if attempt >= 2 {
    compressed := compressRetryContext(out, filesReadPrevAttempt, editsApplied)
    retryErrorMsg += compressed
}

func compressRetryContext(prevResult map[string]any, filesRead, editsApplied []string) string {
    var sb strings.Builder
    sb.WriteString("\n\n## Previous Attempt Summary\n")
    sb.WriteString(fmt.Sprintf("Files read: %s\n", strings.Join(filesRead, ", ")))
    sb.WriteString(fmt.Sprintf("Edits applied: %s\n", strings.Join(editsApplied, ", ")))
    if content, _ := prevResult["content"].(string); content != "" {
        // Truncate to 500 chars
        if len(content) > 500 {
            content = content[:500] + "..."
        }
        sb.WriteString(fmt.Sprintf("Last reasoning: %s\n", content))
    }
    return sb.String()
}
```

---

## Migration Strategy

All changes are **backward-compatible**. No database schema changes required. The changes modify runtime behavior within existing interfaces.

### Phase 1 (Done): Salvage Hardening
- ✅ Force retry on partial salvage
- ✅ Force retry on missing summary
- ✅ Force retry on missing diff
- ✅ Fix log console paused/naming issues

### Phase 2: Checkpoint & Context
- Empty checkpoint validation
- Pre-hydrated context injection
- Discovery budget cap

### Phase 3: Runtime Intelligence
- Hard tool gating
- Negative memory
- Prompt compression

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Pre-hydrated context exceeds token limits | Hard cap at 4,000 tokens with truncation |
| Discovery cap blocks legitimate exploration | Cap is on consecutive reads (resets on write), not total |
| Hard tool gating breaks edge cases | Only block in FAILED/SALVAGED states; DISCOVERY still advisory |
| Prompt compression loses critical context | Always include files_read list and last reasoning summary |
