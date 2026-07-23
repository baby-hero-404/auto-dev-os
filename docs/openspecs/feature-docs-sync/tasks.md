# Tasks: Feature Docs Sync

> Làm sớm (song song Wave 0) — càng muộn càng nhiều nợ docs.

## 1. Guard script — DROPPED (scope-reduced 2026-07-23)

- [ ] ~~1.1 `scripts/docs_freshness.py`: frontmatter parse, git timestamps, stale/untracked report, `--strict`, `--update-readme` (REQ-001/002)~~ — dropped: one-off automation not worth maintaining for this project; frontmatter freshness (`sources:`/`verified:`) is tracked manually instead (see §2)
- [ ] ~~1.2 Tests (repo fixture tạm hoặc mock git)~~ — dropped, no script to test
- [ ] ~~1.3 CI job (fetch-depth 0, warning mode) + hook vào `checklist.py` Audit (REQ-003)~~ — dropped, no CI job without the script

## 2. Trả nợ frontmatter

- [x] 2.1 Thêm `sources:` cho 20 docs hiện có trong `docs/features/` (product 13, engineering 5, hardening 1, README) — done, verified via grep
- [x] 2.2 Commit cột `verified:` per-file (manual, không qua `--update-readme` vì script bị bỏ) (REQ-004, scope-reduced)

## 3. Convention cho spec sets

- [x] 3.1 Bổ sung mục "Docs sync" vào tasks.md của các spec sets — verified: 41/41 sets under docs/openspecs/*/tasks.md now contain a "Docs sync" section (grep confirmed 2026-07-23), covering well beyond the original 13-set mapping (REQ-005)
- [x] 3.2 Ghi convention vào `features/README.md` — verified: `docs/features/README.md:77` states the "Docs Sync" convention (spec must declare impacted docs; docs must carry `sources:` frontmatter); ROADMAP-cli-execution-engine.md:71 also notes it

## 4. Docs sync (chạy dần theo từng wave ship — không làm trước)

- [x] 4.1 Wave 1 ship → viết `product/14-execution-engine.md` (mới) + sửa `product/06`, `product/07`, `product/08` theo mapping — done 2026-07-23
- [x] 4.2 Docs sync cho 14 spec sets đã ship nhưng chưa map (toàn bộ backlog, không chỉ wave kế tiếp) — done 2026-07-23: `product/01,04,05,06,07,08,09,10,13`, `engineering/01,02` cập nhật + `README.md` bump freshness table; xem git diff `docs/features/` để review chi tiết
- [ ] 4.3 ~~Khi untracked = 0 và nợ trả xong → bật `--strict` trong CI~~ — N/A, no CI guard (script dropped)
