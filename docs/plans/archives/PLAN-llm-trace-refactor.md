# PLAN: LLM Trace Chronological Refactor

**Source:** Flow audit — review-fix cycle traceability gap  
**Goal:** Refactor LLM trace directory layout from step-grouped `logs/llm/{step}/call-{n}/` to chronological `logs/llm/call-{n}-{step}/` enabling immediate flow visibility across loopback cycles.  
**Status:** 🟡 In Progress  
**Estimated effort:** Small (3 files + tests)

---

## Problem Statement

Khi task chạy vòng lặp review-fix (vd: `plan → code_be → review → fix → review → fix → test → pr`):

**Before (grouped by step — bad):**
```
logs/llm/
├── plan/call-1/
├── code_backend/call-1/
├── review/call-1/         ← cycle 1? retry?
├── review/call-2/         ← cycle 2? retry?
├── fix/call-1/
└── fix/call-2/
```
❌ Không thể nhìn ra thứ tự thực thi. `review/call-2` có thể là JSON retry hoặc cycle 2.

**After (chronological — good):**
```
logs/llm/
├── call-001-context_load/
├── call-002-analyze/
├── call-003-plan/
├── call-004-code_backend/
├── call-005-review/        ← cycle 1 review
├── call-006-fix/           ← cycle 1 fix
├── call-007-review/        ← cycle 2 review — CLEAR!
├── call-008-fix/           ← cycle 2 fix — CLEAR!
├── call-009-test/
└── call-010-pr/
```
✅ `ls` hiện ra đúng thứ tự thực thi. Nhìn thấy ngay flow có loop.

---

## Affected Files

| # | File | Change | Scope |
|---|:-----|:-------|:------|
| 1 | `server/internal/orchestrator/llm_trace.go` | Refactor path format + enhance metadata | Core |
| 2 | `server/internal/orchestrator/llm_trace_test.go` | NEW — full test suite | Test |
| 3 | `docs/features/5.6-task-system.md` | Update spec docs | Docs |
| 4 | `docs/plans/archives/PLAN-phase5-workspace-structure.md` | Update spec docs | Docs |

---

## Implementation Steps

### Step 1: Refactor `writeLLMCallTrace()` path logic ✅

**Current code (llm_trace.go:37-55):**
```go
stepTraceDir := filepath.Join(ws.Root, "logs", "llm", stepID)
_ = os.MkdirAll(stepTraceDir, 0o755)
callNumber := 1
if files, err := os.ReadDir(stepTraceDir); err == nil {
    // scan for "call-{n}" dirs → increment
}
callDirName := fmt.Sprintf("call-%d", callNumber)
callPath := filepath.Join(stepTraceDir, callDirName)
```

**New code:**
```go
traceRoot := filepath.Join(ws.Root, "logs", "llm")
_ = os.MkdirAll(traceRoot, 0o755)
callNumber := 1
if files, err := os.ReadDir(traceRoot); err == nil {
    // scan ALL "call-{n}-*" dirs across all steps → global increment
}
callDirName := fmt.Sprintf("call-%03d-%s", callNumber, stepID)
callPath := filepath.Join(traceRoot, callDirName)
```

Key changes:
- Path: `logs/llm/{step}/call-{n}/` → `logs/llm/call-{NNN}-{step}/`
- Counter scope: per-step → **global** across all steps
- Padding: `call-1` → `call-001` (3-digit zero-padded for `ls` sort correctness)

### Step 2: Enhance `TraceMetadata` struct

**Current:**
```go
type TraceMetadata struct {
    Model        string    `json:"model"`
    PromptTokens int       `json:"prompt_tokens"`
    OutputTokens int       `json:"output_tokens"`
    AgentID      string    `json:"agent_id"`
    AgentName    string    `json:"agent_name"`
    Role         string    `json:"role"`
    Timestamp    time.Time `json:"timestamp"`
}
```

**New (add `step` + `call_number`):**
```go
type TraceMetadata struct {
    Step         string    `json:"step"`
    CallNumber   int       `json:"call_number"`
    Model        string    `json:"model"`
    PromptTokens int       `json:"prompt_tokens"`
    OutputTokens int       `json:"output_tokens"`
    AgentID      string    `json:"agent_id"`
    AgentName    string    `json:"agent_name"`
    Role         string    `json:"role"`
    Timestamp    time.Time `json:"timestamp"`
}
```

### Step 3: Create comprehensive test suite

**Test cases:**
1. `TestWriteLLMCallTrace_CreatesCorrectDirectory` — verify `call-001-{step}/` format
2. `TestWriteLLMCallTrace_GlobalIncrement` — call with different steps, verify numbering is global
3. `TestWriteLLMCallTrace_ReviewFixLoop` — simulate review→fix→review, verify call-001-review, call-002-fix, call-003-review
4. `TestWriteLLMCallTrace_WritesAllFiles` — verify all 6 files are created (request.json, response.json, prompt.md, output.md, parsed.json, metadata.json)
5. `TestWriteLLMCallTrace_MetadataContainsStep` — verify metadata.json includes `step` and `call_number`
6. `TestWriteLLMCallTrace_SecretsRedacted` — verify tokens in prompt are redacted
7. `TestWriteLLMCallTrace_ResumeAfterRestart` — verify numbering continues from existing dirs after worker restart

### Step 4: Update spec docs

Update `5.6-task-system.md` L193 and `PLAN-phase5-workspace-structure.md` L127 to reflect new format.

---

## Review Checklist

- [ ] Path format: `logs/llm/call-{NNN}-{step}/` with 3-digit zero-padding
- [ ] Global call numbering across all steps (not per-step)
- [ ] `metadata.json` includes `step` and `call_number` fields
- [ ] `ls` output shows correct chronological execution order
- [ ] Secret redaction still works on all files
- [ ] Backward compatible: no crash if old `logs/llm/{step}/` dirs exist
- [ ] All 7 test cases pass
- [ ] Spec docs updated to reflect new layout
- [ ] `go build ./...` passes
- [ ] `go vet ./internal/orchestrator/...` passes
- [ ] Existing orchestrator tests still pass
