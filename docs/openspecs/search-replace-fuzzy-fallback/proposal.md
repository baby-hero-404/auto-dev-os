# Proposal: Search-Replace Fuzzy Fallback (P2.1)

## Why

`ApplySearchReplace` (`server/internal/orchestrator/patch/search_replace.go:97-140`) là exact-match-only: một khác biệt whitespace/indent trong patch LLM sinh ra → `strings.Count` không tìm thấy → `return fmt.Errorf` ngay. Đây là nguồn retry/fail thực tế của patch loop. Aider (reference: `docs/references/agent-platform/DISCOVERY-aider.md`) giải quyết bằng multi-tier fuzzy fallback và đây là fix có scope rõ ràng nhất trong Wave 2.

## What Changes

### Issue 1: Tiered matching pipeline

Thay single exact match bằng chuỗi tier, dừng ở tier đầu tiên match **duy nhất**:

1. **Tier 0 — Exact** (hiện tại, giữ nguyên).
2. **Tier 1 — Trailing-whitespace-normalize**: strip trailing spaces mỗi dòng ở cả search lẫn content trước khi match; apply trên vị trí gốc.
3. **Tier 2 — Relative-indent**: bỏ common leading indent của search block, tìm block content có cùng nội dung sau khi bỏ indent riêng của nó; giữ indent thực tế của content khi replace (re-indent replace block tương ứng).
4. **Tier 3 — Line-trim match**: so từng dòng đã `TrimSpace`; chỉ nhận khi match duy nhất và số dòng bằng nhau.

Không làm diff-match-patch (Tier 4 của Aider) trong phase này — độ phức tạp/rủi ro cao hơn hẳn, để backlog.

### Issue 2: Ambiguity & telemetry

- Match ở nhiều vị trí tại bất kỳ tier nào → fail với message nêu số vị trí + tier (như hành vi exact hiện tại với count > 1).
- Log tier đã dùng cho mỗi apply thành công (đo hiệu quả: bao nhiêu % patch được cứu bởi tier nào).

## Capabilities

### New Capabilities
- Patch apply chịu được lệch whitespace/indent phổ biến của LLM output.
- Metric tier-usage trong logs.

### Modified Capabilities
- `ApplySearchReplace` semantics: từ "exact hoặc chết" sang "tiered, unique-match-required".

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Patch | `server/internal/orchestrator/patch/search_replace.go` (+ file mới `fuzzy.go`) |
| Tests | `search_replace_test.go`, corpus patch lỗi thực tế từ logs |
