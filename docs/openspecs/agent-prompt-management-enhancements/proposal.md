# Proposal: Agent & Prompt Management Enhancements

## Why

Two competitor discovery reports (`docs/references/agent-platform/DISCOVERY-aider.md`, `docs/references/agent-platform/DISCOVERY-hermes-agent.md`) analyzed Aider and Hermes Agent and produced ranked "Applied Takeaways" for Auto Code OS. An Explore agent verified the current codebase against all 8 takeaways; 4 are already implemented or intentionally not worth copying (repo map binary-search pruning, checkpoint/resume, static pricing config, no auto-apply learning). The remaining **4 gaps are confirmed absent** and form the scope of this proposal:

1. `server/internal/orchestrator/patch/search_replace.go::ApplySearchReplace` fails hard on the first `strings.Count(content, search) == 0` — a single whitespace/indent mismatch in an LLM-generated SEARCH block wastes a full retry-LLM round trip instead of being resolved locally.
2. `server/internal/orchestrator/learning/patterns.go::DetectPatterns` is only invoked from `server/internal/orchestrator/worker.go:558-571`, inside a goroutine that fires after the workflow job reaches a terminal status (`Done`/`Failed`). Long-running tasks get zero pattern-detection signal until they finish.
3. `server/internal/context/repomap/ranking.go::CalculatePageRank` already implements an active-file boost (`×50`, lines 100-105) matching Aider's `repomap.py`, but has no equivalent to Aider's `mentioned_idents` (`×10`) — edge weights are never boosted by identifiers that appear in the task description.
4. `server/internal/orchestrator/skills/executor.go` only defines the `SkillCall`/`SkillResult` legacy tool-execution structs. There is no path for an agent to propose a new reusable skill from a just-completed task, even though `models.SuggestionTypeSkill` already exists in `server/pkg/models/learning.go:15` and is unused by any producer.

## What Changes

### Issue 1: Fuzzy Search/Replace Fallback
- Add a fallback chain to `ApplySearchReplace`: exact match (current) → per-line trimmed-whitespace match → relative-indent match → give up with a similarity-based "did you mean" hint (closest matching line range) in the returned error.
- No change to `ParseSearchReplace` block syntax — this only changes how a parsed block is located in file content.

### Issue 2: Mid-Task Learning Nudge
- Add a step-completion counter to the `OnEvent` callback in `worker.go` (same callback that already creates checkpoint commits at line 178-196).
- Every `N` successful steps (configurable, default matches existing `max_reflections`-style constant), fire `LearningEngine.DetectPatterns` asynchronously — same call already used at end-of-task, same goroutine-isolated, non-blocking pattern.
- No change to the suggestion approval flow: `DetectPatterns` already only calls `CreateSuggestion`, which is gated by the existing HITL approval queue.

### Issue 3: Mention-Boost for RepoMap PageRank
- Add a small identifier tokenizer for free-text (task description), distinct from `symbol.ExtractTags` (which parses source files, not prose): extract snake_case/camelCase/kebab-case tokens ≥8 chars.
- Multiply outgoing edge weight for graph nodes whose extracted tags match a mentioned identifier by `10.0`, applied before the power-iteration loop in `CalculatePageRank`, mirroring Aider's `mentioned_idents` factor.
- `CalculatePageRank` gains a new optional parameter; the single caller (`provider.go:312`) is updated to pass the current task description.

### Issue 4: Skill Self-Authoring Suggestion
- Add a new function to the learning package, e.g. `LearningEngine.SuggestSkillFromTask(ctx, task, job)`, invoked from the same end-of-task goroutine in `worker.go` (`finalStatus == models.WorkflowJobStatusDone`) alongside `DetectPatterns`/`SuggestRuleFromErrors`.
- Builds a `CreateSuggestionInput{SuggestionType: models.SuggestionTypeSkill}` summarizing the completed task's tool sequence (reusing memory records already written by `EvaluateOutcome`), status `"proposed"`.
- No auto-apply: the suggestion lands in the existing `LearningService`/HITL approval queue exactly like `pattern`/`rule`/`prompt_patch` suggestions today — this only adds a fourth producer, not a new approval mechanism.

## Capabilities

### New Capabilities
- Fuzzy fallback matching in `ApplySearchReplace` (whitespace-trim, relative-indent, similarity hint on failure)
- Mid-task pattern-detection nudge, triggered by step-completion count instead of only task completion
- Mention-boost weighting in `CalculatePageRank` from task-description identifiers
- Skill-suggestion producer (`SuggestSkillFromTask`) feeding the existing HITL suggestion queue

### Modified Capabilities
- `ApplySearchReplace`: returns a richer error (nearest-match hint) only when all fallback strategies fail
- `worker.go` `OnEvent` callback: gains a step counter and conditional `DetectPatterns` call
- `CalculatePageRank`: new optional `taskDescription string` parameter; existing callers without mentions behave identically (zero-value = no boost)

### Removed Capabilities
- None

## Impact

| Area | Files Affected |
|------|----------------|
| Patch Apply | `server/internal/orchestrator/patch/search_replace.go`, `server/internal/orchestrator/patch/search_replace_test.go` |
| Learning Nudge | `server/internal/orchestrator/worker.go`, `server/internal/orchestrator/learning/patterns.go` |
| Skill Self-Authoring | `server/internal/orchestrator/learning/patterns.go`, `server/internal/orchestrator/worker.go` |
| RepoMap Mention-Boost | `server/internal/context/repomap/ranking.go`, `server/internal/context/repomap/graph_test.go`, `server/internal/context/provider/provider.go` |
