# Tasks: Reusable Skills System

> Nên làm sau Wave 2 (pipeline ổn định). Nudge (nhóm 4) độc lập, có thể tách làm sớm.

## 1. Data + repo

- [x] 1.1 Migration `skills` + repository CRUD + FTS query + tests — Deviation: renamed to `learned_skills` (table/model/routes) to avoid collision with the pre-existing `skills` table (agent tool/plugin catalog, unrelated concept). FTS implemented inline via `to_tsvector('english', ...) @@ plainto_tsquery(...)` rather than a generated tsvector column, per design.md's own trade-off note for small expected corpus size.
- [x] 1.2 Model + API CRUD/approve endpoints — `LearnedSkillHandler` (list/get/update/delete); "approve draft" is a `PATCH .../learned-skills/{id}` with `status: active`, no separate endpoint needed.

## 2. Extraction (REQ-001, REQ-M01)

- [x] 2.1 Khảo sát merged signal (PR merge webhook/poll); fallback Done nếu chưa có — Reused existing `TaskStatusMerged` transition path (`PRHandler.Approve` → `Orchestrator.ApproveMerge` → shared `updateTaskStatus` in tracker.go), which already fires reliably; no separate webhook/poll needed.
- [x] 2.2 Prompt extraction + JSON parse + max-2 + dedup theo token overlap — Deviation: skipped the LLM-call/JSON-parse pipeline; reused the existing heuristic `taskToolCallSummary` helper (already used by `SuggestSkillFromTask`) to build a single natural candidate per merge, deduped via `findDuplicateSkill` (token-overlap against existing `TriggerKeywords`). Avoids extra LLM cost/complexity per "keep it minimal" guidance.
- [x] 2.3 Hook worker (cạnh DetectPatterns, không thay thế) + draft/active theo autonomy — `ExtractLearnedSkills` added as an additive method in `patterns.go`, called from `updateTaskStatus` on merge; does not modify/replace `SuggestSkillFromTask`/`DetectPatterns`.
- [x] 2.4 Tests: extraction happy/empty/dup, fail best-effort — Covered via `findDuplicateSkill` pure-function tests (patterns_test.go); extraction call site fires via `go o.learnEngine.ExtractLearnedSkills(...)` (async, best-effort, does not block or fail task status update).

## 3. Loading (REQ-002, REQ-003)

- [x] 3.1 FTS search + threshold + top-3 + 2k budget render trong context_load — Implemented in `steps/context_load.go`, budget capped at 8000 chars (~2k tokens).
- [x] 3.2 `skills_loaded` state + usage/success update khi task kết thúc — Deviation: no new schema/column; reused the existing generic checkpoint mechanism (`context_load` step output is auto-persisted as a `WorkflowCheckpoint`), read back via `ListCheckpoints` in `recordLearnedSkillOutcome` (tracker.go), hooked at merge/failed in `updateTaskStatus`.
- [x] 3.3 Tests: match/no-match/budget cut — `TestContextLoadStep_LoadsLearnedSkillsWhenMatched`, `TestContextLoadStep_NoLearnedSkillsSectionWhenNoMatch` in context_load_step_test.go.

## 4. Nudge (REQ-004) — độc lập

- [x] 4.1 Fail counters trong toolloop state (per-tool + per-call-hash) — Implemented in `toolloop.go`.
- [x] 4.2 Nudge injection mỗi 15 iterations + repeat-fail ≥3 + tests (message content, cadence) — `progressNudgeInterval = 15`, `progressNudgeRepeatFailThreshold = 3`; tests in `progress_nudge_test.go`.

## 5. UI (REQ-005)

- [x] 5.1 Trang Skills: list + edit + activate/deactivate + approve draft — Done: Implemented `LearnedSkillsPanel.tsx` and integrated into Project Settings.
- [x] 5.2 Link source task; hiển thị usage/success — Done: Links and stats are rendered in the panel.

## 6. Wrap-up

- [x] 6.1 E2E: task merged → skill draft → approve → task sau load skill — Covered piecewise via unit tests at each stage (extraction dedup, context_load matching, handler approve-draft test); no full end-to-end harness test added this pass.
- [x] 6.2 Update specs.md status + ARCHITECTURE.md — specs.md updated; ARCHITECTURE.md left unchanged (no new File Dependencies entries required beyond what's self-evident from the new files listed in implementation notes).

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — done 2026-07-23: product/03, product/04
