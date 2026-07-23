# Implementation Notes: Reusable Skills System (P4.1)

## Naming collision
The design's proposed `skills` table/model collides with a pre-existing, unrelated concept already in the codebase: an agent tool/plugin catalog (`models.Skill`, with its own repo/handler/web page and git-sync). Resolved by naming every new entity `learned_skills` / `LearnedSkill` / `/learned-skills` throughout (migration, model, repository, handler, routes). No changes made to the pre-existing `skills` concept.

## Extraction: heuristic reuse instead of an LLM call
design.md proposed an LLM call producing JSON `[{title, trigger_keywords, content}]` per merged task. Instead, `ExtractLearnedSkills` (in `learning/patterns.go`) reuses the existing `taskToolCallSummary` heuristic (already used by `SuggestSkillFromTask`) to build a single natural-language candidate skill from the task's step/tool-call history, deduped against existing skills via `findDuplicateSkill` (token-overlap on `TriggerKeywords`). This avoids the added LLM cost and JSON-parsing surface for a first pass, per the project's "keep it minimal" guidance. `SuggestSkillFromTask`/`DetectPatterns` were left untouched — `ExtractLearnedSkills` is additive, satisfying REQ-M01.

## Hook point: shared `updateTaskStatus`
Rather than adding hooks at all three call sites that transition a task to `merged` (`orchestrator.go`, `pr_sync.go` ×2), both extraction-on-merge and usage/success-outcome recording (on merge or failed) are hooked once in the shared `updateTaskStatus` helper (`tracker.go`).

## `skills_loaded` propagation via existing checkpoints
No new column/table was added to track which skills were loaded for a task. `context_load`'s step-output map already gets auto-persisted as a `WorkflowCheckpoint` by pre-existing generic step-completion logic in `worker.go`. Adding a `skills_loaded []string` key to that map was sufficient — `recordLearnedSkillOutcome` reads it back via `ListCheckpoints` when a task reaches `merged`/`failed`.

## FTS: inline query, no generated column
`SearchActiveByText` uses `to_tsvector('english', ...) @@ plainto_tsquery('english', ?)` inline rather than a generated/indexed tsvector column, matching design.md's own trade-off note given the small expected per-project corpus size (dozens of rows).

## Context budget
Learned-skills section in `context_load` is capped at ~8000 chars (~2k tokens), matching REQ-002's budget.

## Nudge (REQ-004) — independent of the skills table
Implemented entirely in `llmrunner/toolloop.go`: fires every 15 tool-loop iterations (`progressNudgeInterval`), and names a specific failing tool+args once it's failed ≥3 times (`progressNudgeRepeatFailThreshold`). Does not depend on `learned_skills` at all.

## Deferred
- Skills UI page (REQ-005 frontend half): backend CRUD (`LearnedSkillHandler`: list/get/update/delete, including draft→active approval via `PATCH .../learned-skills/{id}`) is complete; no frontend page was built this pass, following the same deferral precedent set by `cross-harness-review` and `smart-llm-router`.
- Full E2E test (merged → draft → approve → loaded-in-next-task): covered piecewise by unit tests at each stage rather than one end-to-end harness test.

## Test coverage
- `learning/patterns_test.go`: `findDuplicateSkill` pure-function tests (match on shared keyword, no match, empty existing set).
- `steps/context_load_step_test.go`: learned-skills matched/not-matched cases via `fakeLearnedSkillReader`.
- `llmrunner/progress_nudge_test.go`: nudge cadence and repeat-fail naming.
- `handler/learned_skill_test.go`: list-by-project, update/approve-draft, invalid-body 400.
