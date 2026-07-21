# Proposal: Feature Docs Sync (chống outdate cho docs/features)

## Why

`docs/features/` (product/engineering/hardening, 19 docs) là đặc tả tính năng chính nhưng không có cơ chế nào buộc nó cập nhật khi code thay đổi. Roadmap mới (`docs/openspecs/ROADMAP-cli-execution-engine.md`, 13 spec sets) sẽ thay đổi lớn: engine layer mới, flow mới, review semantics mới, router, skills… — nếu không có quy trình sync, `docs/features/` sẽ outdate ngay sau Wave 1 (bằng chứng đã có: mục 1 của Top-10 trong `docs/references/README.md` phải đánh dấu "Outdated as written" vì docs không theo kịp code memory system).

## What Changes

### Issue 1: Docs-impact khai báo trong mỗi OpenSpec set

- Convention mới cho `tasks.md` của mọi spec set: nhóm cuối bắt buộc có mục "Docs sync" liệt kê **đích danh** file `docs/features/**` bị ảnh hưởng (đã áp một phần — chuẩn hóa thành bắt buộc).
- Cập nhật 13 spec sets hiện có: thêm mục docs-sync với mapping cụ thể (vd `pluggable-execution-engine` → sửa `product/08-workflow-engine.md`, thêm `product/14-execution-engine.md`).

### Issue 2: Docs mới cho các tính năng roadmap

- Khi mỗi wave ship: thêm/sửa docs theo mapping (bảng đầy đủ trong design.md). Docs mới viết theo đúng genre convention của `features/README.md` (product = Vietnamese body + prior-art table, engineering = numbered sections).

### Issue 3: Freshness guard tự động

- Script `scripts/docs_freshness.py` (chạy trong CI + `checklist.py` audit): mỗi doc trong `docs/features/` khai frontmatter `sources:` (list glob code paths nó mô tả). Script so `git log -1 --format=%ct` của doc với các source — code mới hơn doc quá N ngày (default 30) và diff đụng sources → cảnh báo "possibly stale" + fail CI ở mức warning-list (không block merge, block được bật sau).
- Doc không có frontmatter → listed là "untracked" để trả nợ dần.

### Issue 4: Trạng thái trong README index

- `features/README.md` Status Legend thêm cột Last-verified (ngày) per doc, do script generate/update — người đọc biết doc nào tin được.

## Capabilities

### New Capabilities
- Freshness guard script + CI wiring; frontmatter sources convention; docs-sync section bắt buộc trong spec sets.

### Modified Capabilities
- 13 spec sets hiện có (thêm docs-sync); `features/README.md` (Last-verified).

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Script | `scripts/docs_freshness.py` |
| CI | workflow hiện có + `scripts/checklist.py` hook |
| Docs | `docs/features/**` (frontmatter), `features/README.md` |
| Specs | `docs/openspecs/*/tasks.md` (13 sets, thêm mục Docs sync) |
