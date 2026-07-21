# Specs: Agent & Prompt Management Enhancements

## Added Requirements

### REQ-001: Whitespace-Tolerant Search/Replace Fallback
> ✅ Status: Implemented

**Scenario:**
- WHEN `ApplySearchReplace` receives an `EditBlock` whose `Search` text matches file content except for leading/trailing whitespace differences on one or more lines
- THEN the exact-match path (`strings.Count`) still runs first and is used if it finds exactly one match
- AND IF the exact match finds zero occurrences, a per-line trimmed-whitespace comparison is attempted against the file content
- AND the edit is applied at the location found by the trimmed comparison, preserving the original file's indentation (not the LLM's)

### REQ-002: Relative-Indent Fallback
> ✅ Status: Implemented

**Scenario:**
- WHEN both exact match and trimmed-whitespace match fail to find the search block
- THEN a relative-indent comparison is attempted: both the search block and each candidate window in the file are re-encoded so each line's indent is expressed relative to the previous line's indent, and compared in that space
- AND IF exactly one candidate window matches in relative-indent space, the edit is applied there using the file's original indentation

### REQ-003: Similarity Hint on Total Failure
> ✅ Status: Implemented

**Scenario:**
- WHEN all fallback strategies (exact, trimmed, relative-indent) fail to locate the search block
- THEN `ApplySearchReplace` returns an error that still contains the file path and original message
- AND the error additionally includes the closest-matching line range found by a similarity search over the file content, to help a subsequent LLM retry target the right location

### REQ-004: Ambiguous Match Safety
> ✅ Status: Implemented

**Scenario:**
- WHEN a fallback strategy (trimmed or relative-indent) finds more than one equally-scored candidate location
- THEN `ApplySearchReplace` does NOT guess — it returns an error indicating the match is ambiguous, same as today's behavior for `count > 1` in exact match

### REQ-005: Mid-Task Pattern Detection Nudge
> ✅ Status: Implemented

**Scenario:**
- WHEN a workflow job's `OnEvent` callback in `worker.go` records a successful step completion
- AND the number of successful steps completed so far in this job is a multiple of the configured nudge interval
- THEN `LearningEngine.DetectPatterns` is invoked asynchronously (non-blocking, same goroutine-isolation pattern as the existing end-of-task call) for the current agent
- AND the main step-execution flow is not delayed or blocked by this call

### REQ-006: Nudge Does Not Duplicate End-of-Task Call
> ✅ Status: Implemented

**Scenario:**
- WHEN a task completes and both a mid-task nudge and the existing end-of-task `DetectPatterns` call would fire on the same step
- THEN the suggestion-creation path in `DetectPatterns` remains idempotent-safe from the caller's perspective — duplicate nudges are acceptable because `CreateSuggestion` always creates a new "proposed" suggestion for HITL review (no auto-apply), consistent with existing behavior for `pattern` suggestions

### REQ-007: Mention-Boost Identifier Extraction
> ✅ Status: Implemented

**Scenario:**
- WHEN `CalculatePageRank` is called with a non-empty task description
- THEN identifiers are extracted from the description using the same heuristic as Aider (snake_case/camelCase/kebab-case tokens with length ≥ 8 characters)
- AND graph edges whose target node's tags contain a matching identifier receive an edge-weight multiplier of `10.0` before the power-iteration loop runs

### REQ-008: Mention-Boost Backward Compatibility
> ✅ Status: Implemented

**Scenario:**
- WHEN `CalculatePageRank` is called with an empty or missing task description (existing callers/tests)
- THEN no mention-boost is applied and PageRank output is identical to current behavior (byte-for-byte score parity with existing `graph_test.go` assertions)

### REQ-009: Skill Suggestion from Completed Task
> ✅ Status: Implemented

**Scenario:**
- WHEN a workflow job reaches `WorkflowJobStatusDone`
- THEN `LearningEngine.SuggestSkillFromTask` is invoked in the same end-of-task goroutine as `DetectPatterns`/`SuggestRuleFromErrors`
- AND it creates a `CreateSuggestionInput` with `SuggestionType: models.SuggestionTypeSkill`, summarizing the task's tool-call sequence and outcome, only when the task's memory record indicates a non-trivial multi-step tool sequence (avoid proposing a "skill" for a single-tool task)

### REQ-010: Skill Suggestion Requires HITL Approval
> ✅ Status: Implemented

**Scenario:**
- WHEN a skill suggestion is created by `SuggestSkillFromTask`
- THEN it is written with the same "proposed" status as all other suggestion types and does NOT modify `server/internal/orchestrator/skills/executor.go` behavior or any live skill registry
- AND no skill becomes callable by an agent until an operator approves it through the existing `LearningService` approval flow

## Modified Requirements

### REQ-M01: `CalculatePageRank` Signature
> ✅ Status: Implemented

**Scenario:**
- WHEN any caller invokes `CalculatePageRank`
- THEN the function accepts a new parameter for task description/mentioned identifiers in addition to `activeFiles`
- AND the sole existing production caller (`server/internal/context/repomap/provider.go:312`) is updated to pass the current task's description text

## Removed Requirements
- None
