---
sources:
  - "server/**"
  - "web/src/components/projects/project-profile.tsx"
  - "server/pkg/llm/anthropic.go"
  - "server/internal/orchestrator/llmrunner/outputfilter/**"
verified: 2026-07-23
---

# 01. Unified AI Gateway

**Status:** 🟡 In Progress (baseline implemented; hardening planned)  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/pkg/llm` (router, providers), `server/internal/service/credential_pool.go`, `server/internal/service/provider_model.go`, `web/src/` provider settings UI  
**Blocking decisions:** Final REST API shape for credentials, frontend provider settings UX, audit-log coverage.  
**Acceptance criteria:** Admin can configure multiple provider credentials with model list per Model Level Group (Fast/Balanced/Powerful), route requests by level, and observe usage/cooldown events.

**Mục tiêu:** Cung cấp một lớp trung gian duy nhất giữa AI Agent và các nhà cung cấp LLM (OpenAI, Anthropic, Google...). Agent không bao giờ gọi trực tiếp tới provider — chỉ cần chỉ định mức độ mong muốn (Fast/Balanced/Powerful), Gateway sẽ tự động chọn model, chọn API key, xử lý lỗi và theo dõi chi phí.

---

## Tại Sao Cần Gateway?

Nếu không có Gateway, mỗi Agent phải tự quản lý API key, tự chọn model, tự xử lý rate limit, tự theo dõi chi phí. Gateway giải quyết toàn bộ vấn đề này ở một nơi duy nhất:

- **Agent chỉ cần nói:** "Tôi cần model Balanced" → Gateway lo phần còn lại.
- **Thay đổi provider:** Chỉ cần cấu hình lại Gateway, không cần sửa Agent.
- **Key hết quota:** Gateway tự động chuyển sang key khác, Agent không bị gián đoạn.

---

## A. Quản Lý API Key (Multi-Key Pool)

Hệ thống hỗ trợ lưu trữ **nhiều API key** cho cùng một Provider:

*   **Lưu trữ an toàn:** API Key được mã hoá AES-256-GCM, không bao giờ trả plaintext về frontend hay ghi log.
*   **Chiến lược chọn key (Baseline):** `fill-first` — Dùng key ưu tiên cao nhất cho đến khi lỗi/rate-limit, rồi mới chuyển key tiếp theo.
*   **Rotation strategy planned:** `round-robin` đã có primitive ở server nhưng chưa có cấu hình persisted/API/UI cho Admin chọn.
*   **Cooldown & Recovery:** Lỗi/Rate-limit tự động kích hoạt trạng thái tạm dừng (cooldown) ở cấp độ **Key + Model**. Nếu một model cụ thể của key bị lỗi, key đó vẫn có thể được dùng cho các model khác cùng provider. Khi hết thời gian cooldown, hệ thống tự động kích hoạt lại.
*   **Mỗi Organization** sở hữu không gian key + model config hoàn toàn cách ly.

## B. Phân Nhóm Model (Model Level Groups)

Thay vì buộc Agent chọn một model cụ thể (vd: `gpt-4o`), Admin cấu hình danh sách model theo **3 cấp độ**:

| Level | Dành cho | Ví dụ model |
|:------|:---------|:------------|
| **Fast** | Task đơn giản, cần tốc độ nhanh | `gpt-5.4-nano`, `gpt-5.4-mini`, `gemini-3.1-flash-lite`, `claude-haiku-4.5` |
| **Balanced** | Hầu hết task lập trình thông thường | `gpt-5.4`, `gemini-3.5-flash`, `claude-sonnet-4.6` |
| **Powerful** | Task phức tạp, thiết kế kiến trúc, review chuyên sâu | `gpt-5.5`, `gpt-5.5-pro`, `claude-opus-4.8`, `gemini-2.5-pro` |

Khi thêm Provider mới, hệ thống **tự động tạo danh sách model mặc định** chia vào 3 nhóm. Admin có thể thêm model, bật/tắt model, chỉnh priority, xoá model trên UI; backend API cũng hỗ trợ cập nhật đầy đủ model metadata.

## C. Model Routing & Smart LLM Routing

Khi Agent được cấu hình với một level (ví dụ: "Balanced"):
1.  Gateway tra cứu danh sách model trong nhóm Balanced.
2.  Kết hợp với các API key đang có sẵn và không bị rate-limit cho model đó.
3.  Chọn model có ưu tiên cao nhất (theo `priority` config) và thực hiện request.

**Smart LLM Router Toggle (Implemented):** Hệ thống tích hợp tính năng tự động tối ưu hóa việc phân luồng model dựa trên độ phức tạp của từng tác vụ. Tính năng này được cấu hình thông qua **Smart LLM Routing Toggle** ở Project Settings (UI). Nếu kích hoạt, các task có độ phức tạp thấp sẽ tự động rơi vào nhóm Fast, độ phức tạp trung bình rơi vào Balanced, và phức tạp cao sẽ rơi vào Powerful (giúp tiết kiệm token budget mà không cần tự chuyển level bằng tay).

**Dynamic Fallback Chain (Baseline):** Nếu model ưu tiên cao nhất bị lỗi hoặc hết quota, Gateway tự động chuyển sang model tiếp theo trong cùng level group (dựa trên cấu hình model priority và credential retry) → đảm bảo zero downtime.
*(Lưu ý: Advanced scoring, format translation và token saver hiện mới ở mức Planned).*

## D. Luồng Vận Hành

**Phía Admin:**
```
1. Thêm Provider (OpenAI, Anthropic, Google...)
2. Nhập 1 hoặc NHIỀU API Key → gateway dùng fill-first theo priority hiện tại
3. Cấu hình danh sách Model theo Level Group (Fast / Balanced / Powerful)
4. Tùy chỉnh base URL nếu dùng self-hosted endpoint
```

**Phía Agent (tự động):**
```
1. Agent gửi request với Level = "Balanced"
2. Gateway resolve model + chọn credential tốt nhất cho model đó
3. (Planned) Format Translation & Token Saver
4. Gọi API Provider → nhận response
5. Nếu rate-limit → đưa cặp (key, model) vào cooldown → retry với key/model khác
6. Ghi usage logs (provider, model, tokens, cost, latency)
```

## E. Prompt Caching (Anthropic)

Mọi lời gọi Anthropic qua `pkg/llm` (`anthropic.go`, `ChatWithOptions`) đính `cache_control: {type: "ephemeral"}` vào block cuối cùng của system prompt và block cuối cùng của mảng tools — 2 phần này giữ nguyên nội dung qua mọi vòng tool-loop (không chèn timestamp/task-id động vào phần được cache). Cached input rẻ hơn ~10x so với input thường. Response usage (`cache_creation_input_tokens`, `cache_read_input_tokens`) được đọc và ghi vào token usage tracking (mục G) để đo hit/miss thực tế.

## F. Smart LLM Router & Token Usage Tracking

Ngoài Smart LLM Routing Toggle (mục C), hệ thống ghi nhận chi tiết token usage của **từng LLM call** vào bảng `token_usage` (`task_id, job_id, step_id, provider, model, input/output/cache tokens, cost_estimate`) — dữ liệu này phục vụ trực tiếp Dashboard Cost Card (§10). Ma trận routing theo step-complexity mặc định:

| Step | Model level mặc định |
|:-----|:----------------------|
| `context_load`, `analyze`, `dor_check` (sinh câu hỏi) | `fast` |
| `plan`, `review` | `balanced` |
| `code_*`, `fix` | theo `default_model_level` của project (thường `powerful`) |
| Task `complexity=easy` | hạ mỗi step 1 bậc (floor `fast`) |

## G. Cross-Harness Review & Tool-Output Filtering

- **Cross-Harness Review:** Project cấu hình `review_harness_policy` (`same` | `different_model` | `different_provider`, §06). Review step resolve model/provider khác với model/provider đã code theo policy này; nếu không có lựa chọn thứ 2 được cấu hình → fallback về cùng model + log warning (không chặn pipeline). Metadata `coded_by`/`reviewed_by` (engine + provider + model) được ghi vào step state + task record và hiển thị trong PR description (nền cho Attestation, §09).
- **Tool-Output Filtering:** Trước khi tool result đi vào context (và trước hard-cut 8000 ký tự trong `llmrunner/toolloop.go`), một pipeline filter thuần Go (`llmrunner/outputfilter/`) chạy: dedup dòng lặp liên tiếp (`[repeated N times]`), error-priority truncation (giữ dòng match error/fail/panic + context quanh, cắt phần "im lặng" trước), path-prefix compression, và strip ANSI/control chars. Mỗi tool khai báo profile riêng (build/test → error-priority mạnh; git diff → không dedup; read file → không filter). Metrics `outputfilter: in=X out=Y saved=Z%` được log mỗi lần chạy.

## H. Mở Rộng Planned

| Feature | Mô tả |
|:--------|:------|
| **Format Translation** | Tự động dịch định dạng giữa các provider (OpenAI ↔ Claude ↔ Gemini) |
| **Advanced Fallback Scoring** | Chấm điểm theo health/latency/quota để chọn provider + model tốt nhất thay vì chỉ theo priority baseline |
| **Configurable Rotation Strategy** | Lưu và expose lựa chọn `fill-first` / `round-robin` qua API + UI |

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| 9router | Combo routing, format translation, token saver, 3-tier fallback |
| LLM Key Manager | Multi-key failover, Effective Score routing |
| LiteLLM | Virtual Key architecture, proxy đa provider, budget/quota per key |
| Langfuse | Audit logging, RBAC, lưu trữ API key an toàn |
