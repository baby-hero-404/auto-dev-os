# Tasks: Fix Search/Replace Tool Infinite Loop

## P0 — Critical

### Task 1.1: Actionable error in search_replace tool for non-existent files
> Links to: REQ-001

**File:** `server/internal/tool/tools/search_replace.go`

**Acceptance Criteria:**
- [x] When `search != ""` and file does not exist, return diagnostic with message containing
      `"does not exist"` and `"create_file"` guidance
- [x] When `search == ""` and file does not exist, behavior is unchanged (create file)
- [x] When file exists, behavior is unchanged for both empty and non-empty search
- [x] No raw OS error text (`no such file or directory`) leaks into the diagnostic message
- [x] Unit test covers the non-existent file + non-empty search case
- [x] Unit test covers the non-existent file + empty search case (existing behavior)
- [x] Existing tests still pass

### Task 1.2: Inform LLM about non-existent affected files in prompt
> Links to: REQ-002

**File:** `server/internal/orchestrator/llmrunner/runner.go`

**Acceptance Criteria:**
- [x] When `ReadAffectedFileContent` returns `("", false)`, a `[NEW FILE]` placeholder is
      emitted in the `### Workspace Affected Files ###` section
- [x] Placeholder text includes the display path and `create_file` guidance
- [x] Existing files continue to be injected with their full content (no regression)
- [x] Unit test verifies placeholder injection for missing files
- [x] Unit test verifies existing files are still injected normally

## P1 — High

### Task 2.1: Circuit-breaker in tool loop for repeated identical failures
> Links to: REQ-003, REQ-M02

**File:** `server/internal/orchestrator/llmrunner/toolloop.go`

**Acceptance Criteria:**
- [x] `failureTracker` struct tracks consecutive failures per `tool_name:path` key
- [x] After 2 consecutive identical failures, the tool call is skipped
- [x] A corrective user message is injected instead of the tool result
- [x] Corrective message contains `create_file` guidance
- [x] Tracker resets on any successful tool call to the same key
- [x] Skipped calls do NOT increment the main iteration counter
- [x] Unit test: 3 consecutive failures → 3rd is skipped, corrective message injected
- [x] Unit test: failure → success → failure → failure → 2nd failure skipped
- [x] Existing tool loop tests still pass

### Task 2.2: Overwrite semantics for empty search in patch engine
> Links to: REQ-004

**File:** `server/internal/orchestrator/patch/search_replace.go`

**Acceptance Criteria:**
- [x] When `search == ""`, `content = replace` (overwrite), not `content += replace` (append)
- [x] New file creation (file doesn't exist, search == "") still works correctly
- [x] Existing file with content is fully replaced when search == ""
- [x] Unit test: existing file + empty search → file content is replaced, not appended
- [x] Unit test: new file + empty search → file is created with replace content
- [x] Existing tests still pass (`TestApplySearchReplace`, `TestApplySearchReplace_NewlineNormalization`)

## P2 — Medium

### Task 3.1: Validate overwrite semantics in patch validator
> Links to: REQ-004

**File:** `server/internal/orchestrator/patch/validator.go`

**Acceptance Criteria:**
- [x] `ValidateSearchReplace` handles `search == ""` on existing files without false errors
- [x] Add comment clarifying that `search == ""` means "overwrite entire file"

### Task 3.2: Update coding instruction template with file-creation guidance
> Links to: REQ-002

**File:** `server/internal/prompts/templates/coding_instruction.tmpl`

**Acceptance Criteria:**
- [x] Template includes a note: "If a file in the affected list is marked [NEW FILE], use
      create_file to create it. Do not use search_replace with a non-empty search on files
      that don't exist yet."

## P3 — Low

### Task 4.1: Add integration-level test for the full agentic loop with non-existent files
> Links to: REQ-001, REQ-002, REQ-003

**Acceptance Criteria:**
- [x] Test simulates: LLM receives affected files with one missing → calls create_file → then
      search_replace → succeeds without loop
- [x] Test simulates: LLM ignores guidance → calls search_replace with search != "" on missing
      file → circuit-breaker fires → LLM self-corrects

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
