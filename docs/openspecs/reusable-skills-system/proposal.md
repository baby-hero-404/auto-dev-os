# Proposal: Reusable Skills System + Mid-Task Learning (P4.1 + P4.4)

## Why

Mỗi task hoàn thành chứa tri thức tái dùng được (cách test project này, pattern prompt hiệu quả, chuỗi tool đúng) nhưng hiện bay hơi khi job kết thúc. `learning.DetectPatterns` đã tồn tại nhưng chỉ chạy end-of-task (`orchestrator/worker.go:566`, gated `finalStatus == Done` — verified) và kết quả chưa được đưa trở lại các task sau một cách có cấu trúc. Multica/Hermes/Superpowers (references) đều hội tụ về: trích xuất → lưu skill → nạp lại theo relevance. Gộp P4.4 (per-N-turn nudge của Hermes) vào set này vì cùng đường ống learning.

## What Changes

### Issue 1: skills table + extraction

- Bảng `skills`: `id, project_id, title, trigger_keywords[], content (markdown), source_task_id, usage_count, success_rate, created_at, updated_at`.
- Sau task đạt `merged` (không chỉ Done): extraction step (1 LLM call) đọc job history (steps, fixes, review feedback) → đề xuất 0-2 skill records ("cách chạy test ở repo X", "pattern sửa lỗi Y"). Autonomy supervised → skill ở trạng thái `draft` chờ user approve trong UI; autonomous → active luôn.

### Issue 2: Skill loading trong context

- `context_load`: search skills theo keywords match với task description (BM25 trên trigger_keywords + title; tái dùng memory search infra nếu tiện) → top-3 skills nhét vào context với budget riêng (~2k tokens).
- Track: skill được load vào task nào → khi task merged/failed cập nhật `usage_count`/`success_rate`.

### Issue 3: Mid-task learning nudge (P4.4)

- Trong tool-loop, mỗi N=15 iterations: chèn 1 system nudge tổng kết "những gì đã thử & thất bại" (build từ tool-call history đã có, thuần Go — không LLM call thêm) để chống lặp vòng vô ích — pattern Hermes.
- Nếu cùng tool + cùng args fail ≥3 lần → nudge cảnh báo cụ thể.

### Issue 4: Skills UI

- Trang Skills per-project: list, edit content, activate/deactivate, xem nguồn task.

## Capabilities

### New Capabilities
- Skill extraction/storage/loading vòng kín với approval; mid-task anti-loop nudge; Skills management UI.

### Modified Capabilities
- `learning.DetectPatterns` mở rộng thành extraction pipeline; context_load thêm skills section.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Migration + repo | bảng `skills` + repository |
| Learning | `internal/learning/*`, `orchestrator/worker.go:566` vicinity |
| Context | `steps/context_load.go` (skills section) |
| Tool-loop | `llmrunner/toolloop.go` (nudge injection) |
| API/Web | skills CRUD + approve; trang Skills |
