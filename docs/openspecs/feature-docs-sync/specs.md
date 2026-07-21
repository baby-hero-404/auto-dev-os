# Specs: Feature Docs Sync

## Added Requirements

### REQ-001: Frontmatter sources
> ❌ Status: Not Started

**Scenario:**
- WHEN doc `docs/features/**/*.md` có frontmatter `sources: [server/internal/workflow/**, ...]`
- THEN script parse được; doc không có frontmatter → xếp nhóm "untracked" trong report

### REQ-002: Staleness detection
> ❌ Status: Not Started

**Scenario:**
- WHEN commit gần nhất đụng `sources` của doc mới hơn commit gần nhất của doc quá 30 ngày
- THEN doc xuất hiện trong report "possibly stale" kèm danh sách commits chưa phản ánh
- AND doc được sửa sau code → không bị flag

### REQ-003: CI + checklist integration
> ❌ Status: Not Started

**Scenario:**
- WHEN CI chạy trên PR
- THEN report freshness in ra dạng warning list (không block merge)
- AND `python scripts/checklist.py .` bao gồm mục docs freshness trong audit output

### REQ-004: Last-verified trong README
> ❌ Status: Not Started

**Scenario:**
- WHEN script chạy với `--update-readme`
- THEN bảng trong `features/README.md` có cột Last-verified cập nhật theo ngày commit gần nhất của từng doc

### REQ-005: Docs-sync bắt buộc trong spec sets
> ❌ Status: Not Started

**Scenario:**
- WHEN một OpenSpec set mới được author (convention trong skill/template)
- THEN `tasks.md` có nhóm "Docs sync" liệt kê file docs/features bị ảnh hưởng (hoặc ghi rõ "none — không đổi hành vi user-facing")
- AND 13 sets hiện có đã được bổ sung mục này với mapping cụ thể

## Modified Requirements
- Không có.

## Removed Requirements
- Không có.
