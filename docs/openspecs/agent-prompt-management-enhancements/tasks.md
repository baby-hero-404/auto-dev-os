# Tasks: Agent & Prompt Management Enhancements

## P0 — Critical

### Task 1.1: Trimmed-Whitespace Fallback Match
> Links to: REQ-001, REQ-004

**Acceptance Criteria:**
- [x] `trimmedLineMatch` helper added to `search_replace.go`
- [x] Called only when exact `strings.Count` finds zero matches
- [x] Applies replacement using file's original indentation, not the LLM's search-block indentation
- [x] Returns ambiguous=true (no apply) when >1 candidate window matches
- [x] Unit test: search block with trailing-space-only diff from file content → single match, correctly applied
- [x] Unit test: two identical trimmed candidates in file → ambiguous error, no partial apply

### Task 1.2: Relative-Indent Fallback Match
> Links to: REQ-002, REQ-004

**Acceptance Criteria:**
- [x] `relativeIndentMatch` helper added to `search_replace.go`
- [x] Called only when both exact and trimmed-whitespace match fail
- [x] Re-encodes indentation relative to previous line for both search block and candidate windows before comparing
- [x] Applies at original file indentation when exactly one match found
- [x] Unit test: search block with a uniformly shifted indent level (e.g. +1 tab throughout) vs file → single match, correctly applied
- [x] Unit test: ambiguous relative-indent match → error, no partial apply

### Task 1.3: Similarity Hint on Total Failure
> Links to: REQ-003

**Acceptance Criteria:**
- [x] `nearestSimilarRange` helper added to `search_replace.go`
- [x] Invoked only after exact, trimmed, and relative-indent fallbacks all fail
- [x] Error message includes file path (existing behavior preserved) plus nearest line range + snippet
- [x] Unit test: search block that doesn't exist anywhere in file → error contains a line-range hint pointing at the most similar block

## P1 — High

### Task 2.1: Mid-Task Nudge Counter in `worker.go`
> Links to: REQ-005

**Acceptance Criteria:**
- [x] `successStepCount` counter scoped per job run, incremented in `OnEvent` on `workflow.StepStatusSuccess`
- [x] `learningNudgeInterval` constant defined (default 4)
- [x] `o.learnEngine.DetectPatterns` invoked via `go` + `context.WithoutCancel(ctx)` when `successStepCount % learningNudgeInterval == 0`
- [x] Step-execution flow (`OnEvent` return) is not blocked by the nudge call
- [x] Unit/integration test: simulate 8 successful steps, verify `DetectPatterns` called exactly twice mid-task (plus once at end-of-task if `finalStatus == Done`)

### Task 2.2: `SuggestSkillFromTask` Producer
> Links to: REQ-009, REQ-010

**Acceptance Criteria:**
- [x] `SuggestSkillFromTask(ctx, task, job)` added to `learning/patterns.go`
- [x] Reads tool-sequence memories via existing `memorySvc.ListByAgent` (same pattern as `DetectPatterns`)
- [x] Skips suggestion creation when fewer than 3 distinct tool calls found in the task's memory
- [x] Creates `CreateSuggestionInput{SuggestionType: models.SuggestionTypeSkill}` with status "proposed" via existing `suggestionSvc.CreateSuggestion`
- [x] Does not touch `skills/executor.go` or any live skill registry
- [x] Unit test: task memory with 4 distinct tool calls → one skill suggestion created
- [x] Unit test: task memory with 1 tool call → no suggestion created

### Task 2.3: Wire `SuggestSkillFromTask` into `worker.go`
> Links to: REQ-009

**Acceptance Criteria:**
- [x] Called in the same end-of-task goroutine as `DetectPatterns`/`SuggestRuleFromErrors`, gated on `finalStatus == models.WorkflowJobStatusDone`
- [x] Does not alter existing `EvaluateOutcome`/`DetectPatterns`/`SuggestRuleFromErrors`/`SuggestPromptPatch` call ordering or error handling
- [x] Integration test: task completes successfully with qualifying tool sequence → skill suggestion appears in suggestion queue after job finishes

## P2 — Medium

### Task 3.1: Mention Identifier Extraction
> Links to: REQ-007

**Acceptance Criteria:**
- [x] `ExtractMentionedIdents(text string) map[string]bool` added to `repomap` package
- [x] Matches snake_case, camelCase, and kebab-case tokens with length >= 8
- [x] Unit test: description "fix the CalculatePageRank function in ranking module" → extracts `CalculatePageRank`, `ranking` excluded (length < 8, "ranking" is 7 chars so also excluded — verify boundary case explicitly)

### Task 3.2: Wire Mention-Boost into `CalculatePageRank`
> Links to: REQ-007, REQ-008, REQ-M01

**Acceptance Criteria:**
- [x] `CalculatePageRank` signature updated to `(activeFiles []string, taskDescription string)`
- [x] `boostedWeight(u, v, mentionedSet)` helper centralizes the `×10.0` multiplier, used consistently in both `outWeightSum` computation and inbound-flow computation
- [x] Empty `taskDescription` produces byte-identical scores to current behavior (REQ-008)
- [x] `graph_test.go` existing call sites updated to pass `""`, still pass unchanged
- [x] New unit test: task description mentioning an identifier present in a low-connectivity node's tags → that node's final rank increases relative to the no-mention baseline

### Task 3.3: Update `provider.go` Caller
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] `provider.go:312` passes the current task's description text as the new `taskDescription` argument
- [x] Integration test: repo map generation for a task with a descriptive title/description biases ranked tags toward mentioned identifiers, verified via existing repo-map golden-style test pattern if one exists, else a new targeted test

## P3 — Low

(none)

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
