# Tasks: LLM Gateway Budget Guard + Consolidation

## P0 — safety gap

### Task 1.1: Extract `CheckBudget`/`CheckActualUsage` thành hàm dùng chung
- [x] File mới `pkg/llm/budget.go`: `CheckBudget`, `CheckActualUsage` theo design.md.
- [x] `Gateway.checkBudget`/`checkActualUsage` (`router.go`) trở thành wrapper gọi hàm mới — không đổi hành vi, có test cũ bảo vệ (chạy lại test hiện có sau refactor để confirm byte-identical).
- Satisfies: REQ-001 (nền tảng)

### Task 1.2: Wiring `CheckBudget`/`CheckActualUsage` vào `AIGateway`
- [x] Đọc trực tiếp `g.cfg.LLM.CircuitMaxTokens`/`CircuitMaxCostUSD` qua helper `budgetLimits(opts)` (không cần field riêng — `AIGateway` đã có sẵn `cfg *config.Config`).
- [x] Chèn `llm.CheckBudget` trong `ChatWithOptions` ngay sau khi `entries` được resolve, trước khi tách eligible/excluded.
- [x] Chèn `checkActualUsage` (helper mới) tại cả 2 điểm trả response thành công (`eligibleEntries` và `excludedEntries` fallback) — check 1 lần trên response cuối cùng, không đụng vào logic retry/cooldown lồng bên trong `attempt()`.
- [x] `gateway_test.go`: `TestAIGatewayChatWithOptions_BudgetBlocksLargePromptBeforeCall`, `TestAIGatewayChatWithOptions_BudgetBlocksOverBudgetResponse`, `TestAIGatewayChatWithOptions_NoConfigMeansNoBudgetLimit`.
- Satisfies: REQ-001

### Task 1.3: Sửa recorder bị rớt ở fallback path
- [x] `pkg/llm/provider.go`: thêm `NewProviderWithRecorder(cfg, recorder)`, `NewProvider` giữ nguyên signature cũ gọi vào với `nil` (không breaking existing callers).
- [x] `cmd/api/main.go`: cả 2 call site trong `buildLLMProvider` đổi sang `NewProviderWithRecorder(cfg, recorder)` — grep xác nhận đây là 2 call site duy nhất trong `server/`.
- [x] Test: `TestNewProviderWithRecorder_ThreadsRecorderIntoGatewayFallback`.
- Satisfies: REQ-M01

## P2 — dọn dead code cùng khu vực

### Task 2.1: `EstimateMessageTokens` — **không xoá** (kế hoạch đổi sau khi implement Task 1.2)
- [x] Re-verify trước khi xoá theo đúng quy trình — phát hiện Task 1.2 giờ khiến `AIGateway.ChatWithOptions` gọi `llm.EstimateMessageTokens(messages)` (cross-package, cần bản exported) để ước tính input token cho `CheckBudget`. Hàm **không còn dead** sau khi OpenSpec này implement xong chính nó — huỷ xoá, giữ nguyên.
- Satisfies: REQ-R01 (huỷ — xem ghi chú ở specs.md)

### Task 2.2: `ComboEntry.Priority` — đã thêm comment
- [x] Xác nhận field chỉ dùng để log/debug (0 nơi đọc để routing, ordering đến từ SQL `ORDER BY priority ASC`) — thêm doc comment giải thích rõ trên `ComboEntry` (`pkg/models/provider_model.go`), không xoá field (giữ nguyên shape JSON).
- Satisfies: (dọn dẹp, không có REQ riêng)

### Task 2.3: `providerFromCredential` thiếu case `"gateway"` — xác nhận không reachable, không cần sửa
- [x] Kiểm tra toàn bộ UI tạo credential (`AddApiCredentialModal.tsx`, `AddCredentialModal.tsx`, `AddCliCredentialModal.tsx`, `ModelRoutingRules.tsx`, `ai-providers/page.tsx`) — tất cả đều `filter(p => p !== "gateway")`, không có đường nào tạo credential với `provider="gateway"` qua UI.
- [x] Backend `providerFromCredential`'s nhánh `default` đã trả lỗi rõ ràng ("unsupported provider") thay vì crash nếu case này từng xảy ra qua API trực tiếp — chấp nhận được như lưới an toàn, không cần thêm case riêng.
- Satisfies: (bonus correctness — xác nhận không cần sửa)

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-001 | 1.1, 1.2 |
| REQ-M01 | 1.3 |
| REQ-R01 | 2.1 |
