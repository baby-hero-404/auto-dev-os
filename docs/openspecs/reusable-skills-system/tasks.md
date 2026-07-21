# Tasks: Reusable Skills System

> Nên làm sau Wave 2 (pipeline ổn định). Nudge (nhóm 4) độc lập, có thể tách làm sớm.

## 1. Data + repo

- [ ] 1.1 Migration `skills` + repository CRUD + FTS query + tests
- [ ] 1.2 Model + API CRUD/approve endpoints

## 2. Extraction (REQ-001, REQ-M01)

- [ ] 2.1 Khảo sát merged signal (PR merge webhook/poll); fallback Done nếu chưa có
- [ ] 2.2 Prompt extraction + JSON parse + max-2 + dedup theo token overlap
- [ ] 2.3 Hook worker (cạnh DetectPatterns, không thay thế) + draft/active theo autonomy
- [ ] 2.4 Tests: extraction happy/empty/dup, fail best-effort

## 3. Loading (REQ-002, REQ-003)

- [ ] 3.1 FTS search + threshold + top-3 + 2k budget render trong context_load
- [ ] 3.2 `skills_loaded` state + usage/success update khi task kết thúc
- [ ] 3.3 Tests: match/no-match/budget cut

## 4. Nudge (REQ-004) — độc lập

- [ ] 4.1 Fail counters trong toolloop state (per-tool + per-call-hash)
- [ ] 4.2 Nudge injection mỗi 15 iterations + repeat-fail ≥3 + tests (message content, cadence)

## 5. UI (REQ-005)

- [ ] 5.1 Trang Skills: list + edit + activate/deactivate + approve draft
- [ ] 5.2 Link source task; hiển thị usage/success

## 6. Wrap-up

- [ ] 6.1 E2E: task merged → skill draft → approve → task sau load skill
- [ ] 6.2 Update specs.md status + ARCHITECTURE.md
