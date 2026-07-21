# Proposal: Review Verdict Split (P2.3)

## Why

`server/internal/prompts/steps/review.md` hiện có 1 verdict duy nhất (confirmed via grep — không có `spec_compliance`/`code_quality` fields). Hệ quả: fail vì "code không đúng yêu cầu" và fail vì "code xấu/thiếu test" đều đi chung một đường fix, dù cách xử lý đúng phải khác nhau. Superpowers (reference) tách 2 verdict và route khác nhau — spec-compliance fail nghiêm trọng hơn (quay lại plan/code với context spec), quality fail nhẹ hơn (fix tại chỗ).

## What Changes

### Issue 1: Structured review output

- `review.md` prompt yêu cầu output JSON: `{spec_compliance: {verdict: pass|fail, violations: []}, code_quality: {verdict: pass|fail, issues: []}, summary}`.
- Parser trong `steps/review.go` đọc structured output (theo pattern parser hiện có của analyze — `analyze_parser.go`).

### Issue 2: Differential routing

- `spec_compliance=fail` → fix step nhận **violations + spec gốc** làm instruction chính, ưu tiên trước quality issues; nếu fail lần 2 liên tiếp vì cùng violation → escalate (pause cho user, thay vì đốt hết `MaxReviewFixCycles`).
- Chỉ `code_quality=fail` → fix step như hiện tại với issues list.
- Cả hai pass → sang test/pr như cũ.

### Issue 3: UI

- Task detail review panel hiển thị 2 verdict riêng (badge Spec / Quality) + danh sách violations/issues.

## Capabilities

### New Capabilities
- 2-axis review verdict + escalation khi spec violation lặp.

### Modified Capabilities
- Review output schema; fix step instruction assembly; review→fix routing trong worker.

### Removed Capabilities
- Single-verdict field (thay bằng structured — giữ backward-compat khi parse fail: coi như single verdict cũ).

## Impact

| Area | Files Affected |
|------|----------------|
| Prompts | `server/internal/prompts/steps/review.md` |
| Steps | `server/internal/orchestrator/steps/review.go`, `fix.go` |
| Worker | routing/escalation trong `orchestrator/worker.go` |
| Web | review panel trong task detail |
