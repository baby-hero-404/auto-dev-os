# Proposal: Tool-Output Filtering Pipeline (P2.4)

## Why

Verified 2026-07-20: không có bất kỳ lớp lọc nội dung nào cho tool results — giới hạn duy nhất là hard-cut 8000 chars trong `orchestrator/llmrunner/toolloop.go:44-58` (`maxToolResultChars`). Build log 50KB bị chém mù ở 8000 chars có thể mất chính dòng error; log lặp 1000 dòng giống nhau chiếm chỗ vô ích. rtk/claw-compactor (references) chứng minh content-aware filtering giảm 40-80% token trên tool output — đây là gap token-compression lớn nhất của project. Exit-code đã tách riêng khỏi output text (verified) nên nén output không rủi ro hỏng status semantics.

## What Changes

### Issue 1: Filter pipeline trước hard-cut

Package `server/internal/orchestrator/llmrunner/outputfilter/` — chuỗi filter thuần Go (không LLM), chạy trước `maxToolResultChars`:

1. **Dedup dòng lặp**: N dòng identical liên tiếp → 1 dòng + `[repeated N times]`.
2. **Error-priority truncation**: khi phải cắt, giữ ưu tiên: dòng match error patterns (error/fail/panic/FAIL/✗/warning) + K dòng context quanh chúng + đầu/cuối output; cắt phần "giữa im lặng" trước.
3. **Path prefix compression**: đường dẫn tuyệt đối lặp lại nhiều lần → rút về relative sau lần đầu.
4. **ANSI/control strip**: mã màu terminal, carriage-return progress bars.

### Issue 2: Per-tool profiles

- Mỗi tool khai báo profile: `build/test` → error-priority mạnh; `git diff` → không dedup (mỗi dòng có nghĩa), chỉ cắt đuôi file quá dài; `read file` → không filter (đã có bound riêng).
- Default profile an toàn (strip ANSI + dedup) cho tool không khai báo.

### Issue 3: Metrics

- Log `outputfilter: in=X out=Y saved=Z%` mỗi lần filter chạy — đo hiệu quả thật cho roadmap.

## Capabilities

### New Capabilities
- Content-aware filtering pipeline với per-tool profiles + savings metrics.

### Modified Capabilities
- `toolloop.go` áp filter trước hard-cut (hard-cut giữ làm safety net cuối).

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| New pkg | `server/internal/orchestrator/llmrunner/outputfilter/*.go` |
| Tool loop | `server/internal/orchestrator/llmrunner/toolloop.go` |
| Tools | khai báo profile trong `server/internal/tool/tools/*.go` (metadata, không đổi logic) |
