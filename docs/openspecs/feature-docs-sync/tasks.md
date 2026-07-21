# Tasks: Feature Docs Sync

> Làm sớm (song song Wave 0) — càng muộn càng nhiều nợ docs.

## 1. Guard script

- [ ] 1.1 `scripts/docs_freshness.py`: frontmatter parse, git timestamps, stale/untracked report, `--strict`, `--update-readme` (REQ-001/002)
- [ ] 1.2 Tests (repo fixture tạm hoặc mock git)
- [ ] 1.3 CI job (fetch-depth 0, warning mode) + hook vào `checklist.py` Audit (REQ-003)

## 2. Trả nợ frontmatter

- [ ] 2.1 Thêm `sources:` cho 19 docs hiện có trong `docs/features/` (product 13, engineering 5, hardening 1)
- [ ] 2.2 Chạy `--update-readme` lần đầu, commit cột Last-verified (REQ-004)

## 3. Convention cho spec sets

- [ ] 3.1 Bổ sung mục "Docs sync" vào tasks.md của 13 spec sets theo bảng mapping trong design.md (REQ-005)
- [ ] 3.2 Ghi convention vào `features/README.md` + note trong ROADMAP để spec set tương lai tuân theo

## 4. Docs sync (chạy dần theo từng wave ship — không làm trước)

- [ ] 4.1 Wave 1 ship → viết `product/14-execution-engine.md` + sửa product/06,07,08 theo mapping
- [ ] 4.2 Mỗi wave sau: thực hiện mục Docs sync của các set vừa ship, bump `verified:`
- [ ] 4.3 Khi untracked = 0 và nợ trả xong → bật `--strict` trong CI
