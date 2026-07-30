# Specs: LLM Gateway Budget Guard + Consolidation

## Added Requirements

### REQ-001: `AIGateway` enforce budget circuit breaker giống `pkg/llm.Gateway`
> ✅ Status: Fully Implemented

**Scenario: Ước tính input token vượt `LLM_CIRCUIT_MAX_TOKENS`**
- WHEN `AIGateway.ChatWithOptions` được gọi với `opts.OrgID != ""` (đường chính, không phải fallback), input messages ước tính vượt `cfg.LLM.CircuitMaxTokens`
- THEN request bị chặn **trước khi** gọi provider thật — trả lỗi `ErrCircuitOpen`-wrapped, không tốn token/tiền gọi API

**Scenario: Cost ước tính vượt `LLM_CIRCUIT_MAX_COST_USD`**
- WHEN input+output-limit ước tính cost vượt `cfg.LLM.CircuitMaxCostUSD`
- THEN request bị chặn trước khi gọi provider, cùng cơ chế trên

**Scenario: Response thật vượt giới hạn (post-call check)**
- WHEN provider trả response có `PromptTokens`/`OutputTokens`/cost thật vượt giới hạn đã cấu hình
- THEN lỗi `ErrCircuitOpen` được trả về cho caller (không silently accept response vượt ngân sách) — hành vi post-call check giống `pkg/llm.Gateway.checkActualUsage` hôm nay

**Scenario: `LLM_CIRCUIT_MAX_TOKENS`/`LLM_CIRCUIT_MAX_COST_USD` = 0 (không cấu hình)**
- WHEN cả 2 biến env không set (giá trị 0, default)
- THEN không có giới hạn nào được áp — hành vi y hệt hôm nay khi dùng `AIGateway` (không breaking change cho org chưa cấu hình circuit breaker)

## Modified Requirements

### REQ-M01: `buildLLMProvider`'s fallback path giữ usage recorder
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `LLM_PROVIDER=gateway` được set và `AIGateway.chatFallback` gọi vào `pkg/llm.Gateway` (org-less call)
- THEN usage/cost của call đó vẫn được ghi nhận qua recorder — không còn rớt âm thầm như hôm nay (`recorder=nil` hardcode)

## Removed Requirements

### REQ-R01: `EstimateMessageTokens` — **huỷ, không xoá**
> 🚫 Status: Cancelled — implementing REQ-001 gave this function a real cross-package caller

**Ghi chú:** `AIGateway.ChatWithOptions` (REQ-001's implementation) giờ gọi `llm.EstimateMessageTokens(messages)` để ước tính input token cho `CheckBudget` — hàm không còn 0-caller sau khi chính OpenSpec này được implement. Giữ nguyên, không xoá. Đây là ví dụ cụ thể của quy trình "re-verify ngay trước khi xoá" trong `design.md` phát huy tác dụng.
