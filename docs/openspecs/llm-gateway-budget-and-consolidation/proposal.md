# Proposal: LLM Gateway Budget Guard + Consolidation

## Why

Backend có **2 implementation độc lập** cho cùng 1 việc "gọi LLM với fallback-chain/retry giữa nhiều credential":
- `pkg/llm.Gateway` (`router.go`) — implementation cũ, đầy đủ: fallback chain, exponential backoff retry qua `IsTransientError`, "Harness Independence" (loại credential lỗi rồi fallback êm), **budget circuit breaker** (`checkBudget`/`checkActualUsage`, gate bằng `LLM_CIRCUIT_MAX_TOKENS`/`LLM_CIRCUIT_MAX_COST_USD`, đã document trong `.env.example`).
- `internal/gateway.AIGateway` (`gateway.go`) — implementation mới hơn, org-scoped, tích hợp `CredentialPoolService`/cooldown per-credential. Đây là **con đường chính cho mọi traffic multi-tenant**: `cmd/api/main.go` luôn khởi tạo `credentialPoolSvc` non-nil → `buildLLMProvider` luôn đi nhánh `AIGateway`. `pkg/llm.Gateway` chỉ còn sống khi `LLM_PROVIDER=gateway` được set tường minh (default `.env.example` là `openai`), và ngay cả khi đó cũng chỉ được `AIGateway.chatFallback` gọi tới cho trường hợp cạnh `opts.OrgID == ""`.

Hệ quả nghiêm trọng nhất: **`AIGateway` — con đường chính — không có budget circuit breaker nào cả.** Grep `MaxTokensPerCall|MaxCostUSDPerCall|ErrCircuitOpen` trong `gateway.go`: 0 kết quả. Biến môi trường `LLM_CIRCUIT_MAX_TOKENS`/`LLM_CIRCUIT_MAX_COST_USD` documented trong `.env.example` như một cơ chế bảo vệ đang hoạt động, nhưng trên thực tế **vô hiệu cho toàn bộ traffic tổ chức thật** — chỉ hoạt động trên nhánh gần như không bao giờ được gọi tới. Đây là gap an toàn/chi phí thật, không chỉ dead code.

Bonus, cùng khu vực: `AIGateway`'s fallback path (`llm.NewProvider` với `LLM_PROVIDER=gateway`) hardcode `recorder=nil` khi build `pkg/llm.Gateway` — telemetry/cost tracking biến mất âm thầm trên nhánh đó, và không test nào bắt được (`TestAIGatewayChatFallsBackWithoutOrg` dùng fake provider, không phải `llm.Gateway` thật).

## What Changes

### Issue 1: Thêm budget circuit breaker vào `AIGateway` (P0 — safety gap, không phải cleanup)
- Tái dùng đúng logic `checkBudget`/`checkActualUsage` từ `pkg/llm.Gateway` (không viết lại) — hoặc extract thành hàm dùng chung `pkg/llm.CheckBudget(...)` mà cả 2 gateway gọi, hoặc port trực tiếp logic vào `AIGateway.ChatWithOptions`/`attempt()`. Quyết định cụ thể ở design.md.
- Đọc cùng 2 biến env `LLM_CIRCUIT_MAX_TOKENS`/`LLM_CIRCUIT_MAX_COST_USD` đã có sẵn trong config — không thêm biến mới.

### Issue 2: Sửa `recorder=nil` bị hardcode ở fallback path
- Thread `recorder` (đã có sẵn trong scope của `buildLLMProvider`) xuống `llm.NewProvider`/`NewGatewayFromConfigWithRecorder` thay vì hardcode `nil`.

### Issue 3: Document rõ `pkg/llm.Gateway` là legacy/secondary path (cùng pattern với `ExecutionEngine`/`CLIEngineConfig`)
- Thêm comment ở đầu `router.go` giải thích rõ đây là fallback path cho org-less calls, không phải primary — tránh nhầm lẫn cho người đọc code sau này (đã có 1 comment trong `gateway.go:192-195` tự thừa nhận sự trùng lặp — chính thức hoá thành doc thay vì rải rác trong comment).
- **Không xoá `pkg/llm.Gateway` trong OpenSpec này** — vẫn là fallback path thật cho org-less calls, xoá cần xác nhận riêng không còn call site nào cần nó (theo quyết định breaking-change đã thống nhất: chỉ xoá khi *xác nhận* không dùng, ở đây vẫn đang dùng dù hiếm).

### Issue 4: Dọn dead code nhỏ cùng khu vực (P2, đi kèm cho tiện review 1 lần)
- `pkg/llm/pricing.go:74` `EstimateMessageTokens` — 0 call site, xoá.
- `internal/gateway/gateway.go:387` `ComboEntry.Priority` field — ghi nhưng không đọc (ordering thật sự đến từ SQL `ORDER BY priority ASC`), xoá field hoặc thêm comment giải thích rõ nó chỉ để debug/log, không dùng để routing.
- `internal/gateway/gateway.go:402-414` `providerFromCredential` thiếu case `"gateway"` dù `models.IsAllowedProvider` cho phép lưu credential với `provider="gateway"` — xác nhận đây có phải path thật sự reachable không (kiểm tra UI tạo credential có cho chọn "gateway" làm provider hay không); nếu có, thêm case xử lý hoặc chặn từ UI/validation sớm hơn thay vì fail sâu trong gateway.

## Capabilities

### New Capabilities
- Budget circuit breaker hoạt động trên con đường traffic chính (`AIGateway`), không chỉ nhánh phụ.

### Modified Capabilities
- `buildLLMProvider`'s fallback path giữ đúng usage recorder thay vì rớt âm thầm.

### Removed Capabilities
- `EstimateMessageTokens` (dead export).

## Impact

| Area | Files Affected |
|------|----------------|
| Backend gateway | `server/internal/gateway/gateway.go` |
| Backend llm | `server/pkg/llm/router.go`, `server/pkg/llm/provider.go`, `server/pkg/llm/pricing.go` |
| Backend entrypoint | `server/cmd/api/main.go` (`buildLLMProvider`) |
| Backend tests | `server/internal/gateway/gateway_test.go`, `server/pkg/llm/router_test.go` (nếu có) |
