# PLAN 5.9: Telemetry & Failure Observability

**Status:** Proposed  
**Feature Owner:** `docs/features/5.9-dashboard-analytics.md`, `docs/features/5.1-unified-ai-gateway.md`

## 1. Ngữ Cảnh & Mục Tiêu
Theo cập nhật mới nhất từ tài liệu:
- Dashboard Dashboard Analytics (5.9) yêu cầu hiển thị "failure reasons" lấy từ `workflow_jobs.last_error`.
- Unified AI Gateway (5.1) yêu cầu ghi nhận usage logs (provider, model, key_label, tokens, cost, latency) ở bước cuối cùng của Request.
- Hiện tại, Gateway và Orchestrator mới chỉ dừng ở mức ghi log console/CLI, chưa lưu các thông số này thành dạng metrics có cấu trúc trong Database phục vụ cho Dashboard.

## 2. Gateway Telemetry (Usage Metrics)

**Mục tiêu:** Cập nhật `AIGateway.Chat()` để luôn ghi nhận metrics (thành công hoặc thất bại) vào bảng `token_usage` đã có sẵn trong hệ thống thay vì tạo bảng mới.

**Bổ sung Cấu trúc dữ liệu (`token_usage` migration):**
Bảng `token_usage` đã được tạo từ migration `000006_token_usage.up.sql`. Tuy nhiên, để đáp ứng khả năng phân tích chi tiết theo Organization và theo dõi hiệu suất của từng Credential, cần bổ sung cột `org_id` và `credential_id`.

Tạo file migration `000016_extend_token_usage.up.sql`:
```sql
ALTER TABLE token_usage 
    ADD COLUMN org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    ADD COLUMN credential_id UUID REFERENCES provider_credentials(id) ON DELETE SET NULL;

CREATE INDEX idx_token_usage_org_id ON token_usage(org_id);
CREATE INDEX idx_token_usage_credential_id ON token_usage(credential_id);
```

*(Lưu ý: Mặc dù yêu cầu tại mục 1 là hiển thị `key_label`, DB scheme chỉ cần lưu `credential_id`. API `/api/v1/analytics/token-usage` của backend sẽ thực hiện JOIN với bảng `provider_credentials` (hoặc lookup tại runtime) để trả về `key_label` cho frontend, đảm bảo chuẩn hóa dữ liệu.)*


Tạo file migration `000016_extend_token_usage.down.sql`:
```sql
DROP INDEX IF EXISTS idx_token_usage_credential_id;
DROP INDEX IF EXISTS idx_token_usage_org_id;

ALTER TABLE token_usage 
    DROP COLUMN IF EXISTS credential_id,
    DROP COLUMN IF EXISTS org_id;
```


Cập nhật struct `TokenUsage` trong `server/pkg/models/analytics.go`:
```go
type TokenUsage struct {
	ID           string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID        *string   `json:"org_id,omitempty" gorm:"type:uuid"`
	CredentialID *string   `json:"credential_id,omitempty" gorm:"type:uuid"`
	ProjectID    *string   `json:"project_id,omitempty" gorm:"type:uuid"`
	AgentID      *string   `json:"agent_id,omitempty" gorm:"type:uuid"`
	TaskID       *string   `json:"task_id,omitempty" gorm:"type:uuid"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Tier         string    `json:"tier"`
	PromptTokens int       `json:"prompt_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	LatencyMS    int64     `json:"latency_ms"`
	Status       string    `json:"status"` // 'ok' hoặc 'failed'
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
}
```

**Mở rộng `llm.UsageRecord` và `AnalyticsRepo`:**
Để truyền tải thông tin về Organization và Credential từ Gateway xuống Repository, cần thực hiện:

1. Thêm trường `OrgID` và `CredentialID` vào struct `llm.UsageRecord` tại `server/pkg/llm/router.go`:
```go
type UsageRecord struct {
	ProjectID    string
	AgentID      string
	TaskID       string
	OrgID        string // Trường mới
	CredentialID string // Trường mới
	Provider     string
	Model        string
	Tier         string
	PromptTokens int
	OutputTokens int
	CostUSD      float64
	LatencyMS    int64
	Status       string
	Error        string
}
```

2. Cập nhật phương thức `RecordLLMUsage` trong `server/internal/repository/analytics.go` để lưu trữ 2 trường mới này vào bảng `token_usage` thông qua struct GORM `TokenUsage`.

**Implementation (Gateway Hook):**
- Struct `AIGateway` trong `server/internal/gateway/gateway.go` cần nhận dependency `UsageRecorder` (được triển khai bởi `AnalyticsRepo`).
- Để ghi nhận telemetry chính xác và tránh bỏ sót các điểm return sớm (early return paths) trong `Chat()`, ta sẽ sử dụng pattern **deferred function** kết hợp với **named return values** cho `Chat()`:
  ```go
  func (g *AIGateway) Chat(ctx context.Context, messages []llm.Message) (resp *llm.Response, err error) {
  ```
- Tại đầu hàm `Chat()`, khai báo `defer func()` để bắt kết quả cuối cùng của `resp` và `err` trước khi thoát hàm:
  - **Trường hợp bỏ qua không ghi nhận:** Khi request không có `opts.OrgID` (các cuộc gọi fallback môi trường gốc).
  - **Trường hợp ghi nhận thành công (Status = "ok"):** Khi gọi thành công qua một credential hoặc khi fallback thành công ở cuối hàm.
  - **Trường hợp ghi nhận lỗi (Status = "failed"):** Khi gặp lỗi ở bất kỳ vị trí nào: lỗi validate Virtual Key, lỗi Context cancel, lỗi cạn kiệt route (exhausted routes).
  - Telemetry ghi nhận được thực hiện trong một **async goroutine** riêng biệt để không block tiến trình của API client.

Chi tiết mã nguồn mẫu của Deferred Telemetry Hook:
```go
defer func() {
    if opts.OrgID == "" || g.recorder == nil {
        return
    }

    record := llm.UsageRecord{
        ProjectID: opts.ProjectID,
        AgentID:   opts.AgentID,
        TaskID:    opts.TaskID,
        OrgID:     opts.OrgID,
        LatencyMS: time.Since(startTime).Milliseconds(),
    }

    // Xác định Provider, Model và Tier để ghi nhận
    var provider, model, tier string
    if lastEntry != nil {
        provider = lastEntry.Provider
        model = lastEntry.Model
        tier = lastEntry.Tier
    } else if g.fallback != nil {
        // Fallback telemetry is best-effort. Nếu fallback provider implement llm.MetadataProvider, ta ưu tiên lấy thông tin metadata chuẩn.
        if metaProv, ok := g.fallback.(llm.MetadataProvider); ok {
            meta := metaProv.Metadata()
            provider = meta.Provider
            model = meta.Model
        } else {
            provider = g.fallback.Name()
            if g.cfg != nil {
                model = g.cfg.LLM.Model
            }
        }
    }

    // Nếu provider, model hoặc tier bị rỗng (ví dụ do lỗi validation key hoặc định tuyến trước khi xác định được model),
    // gán các giá trị sentinel để tránh vi phạm ràng buộc NOT NULL của cơ sở dữ liệu.
    if provider == "" {
        provider = "gateway"
    }
    if model == "" {
        model = "unknown"
    }
    if tier == "" {
        tier = "balanced"
    }

    record.Provider = provider
    record.Model = model
    record.Tier = tier

    // Lưu credential ngay cả khi fail để không làm hỏng dữ liệu phân tích failure rate của key
    if lastCred != nil {
        record.CredentialID = lastCred.ID
    }



    if err != nil {
        record.Status = "failed"
        record.Error  = err.Error()
    } else if resp != nil {
        record.Status       = "ok"
        // Sử dụng model thực tế trả về từ response nếu có, ngược lại dùng model cấu hình
        if resp.Model != "" {
            record.Model = resp.Model
            model = resp.Model
        }
        record.PromptTokens = resp.PromptTokens
        record.OutputTokens = resp.OutputTokens
        
        // Tính toán cost sử dụng llm package helpers
        meta := llm.MetadataForModel(provider, model)
        record.CostUSD      = llm.EstimateCost(resp.PromptTokens, resp.OutputTokens, meta)
    }


    // Tạo context mới có timeout nhưng giữ lại các metadata/trace từ request cũ
    ctxCopy := context.WithoutCancel(ctx)
    bgCtx, cancel := context.WithTimeout(ctxCopy, 2*time.Second)

    go func() {
        defer cancel()
        if recErr := g.recorder.RecordLLMUsage(bgCtx, record); recErr != nil {
            log.Printf("[AIGateway] Telemetry record failed: %v", recErr)
        }
    }()

}()
```



## 3. Workflow Engine Failure Reasons (Task Analytics)

**Mục tiêu:** Hiển thị nguyên nhân lỗi (failure reason) của các Task bị thất bại trực tiếp trên UI Dashboard.

**Kiểm chứng Baseline:**
- Trường `last_error TEXT` **đã tồn tại** trong bảng `workflow_jobs` và struct GORM `WorkflowJob` ở codebase hiện tại.
- Trình điều phối `Orchestrator` trong `server/internal/orchestrator/orchestrator.go` cũng **đã tự động bắt lỗi** và cập nhật `last_error` khi workflow chuyển sang trạng thái `failed` hoặc `paused`.
- **Hành động cần làm:** API trả về trạng thái task/workflow (như endpoint `GET /api/v1/tasks/{taskID}/workflow`) đã tự động serialize trường này thông qua JSON tag `last_error` của struct `WorkflowJob`. Frontend chỉ cần đọc trực tiếp trường `job.last_error` để hiển thị trên UI.


## 4. REST API Endpoint cho Dashboard UI

Chúng ta sẽ sử dụng và mở rộng các API Dashboard & Analytics đã có sẵn:
- **Token Usage / Cost Metrics:** Sử dụng endpoint `/api/v1/analytics/token-usage` (thay thế cho `gateway-usage` đề xuất trước đó). Endpoint này đã được định tuyến thông qua `AnalyticsHandler.TokenUsage` và truy vấn trực tiếp từ bảng `token_usage`.
  - **Cập nhật Contract & Query:** Bổ sung trường `KeyLabel string` vào cấu trúc response trả về (`TokenUsageSummary` hoặc tương đương). Cập nhật câu lệnh SQL query group-by trong `AnalyticsRepo.TokenUsage` để thực hiện `LEFT JOIN provider_credentials pc` lấy trường `pc.label AS key_label`, và group thêm theo dimension `credential_id` để Frontend có thể hiển thị thống kê sử dụng trên từng key.
  - **Lưu ý Quan trọng về Bảo mật (Phân quyền Tổ chức):** Hiện tại handler/repo chỉ lọc theo `project_id`. Cần cập nhật `AnalyticsHandler.TokenUsage`, `AnalyticsService` và `AnalyticsRepo.TokenUsage` để bắt buộc nhận và áp dụng bộ lọc theo `org_id` (được trích xuất từ Authentication Context của user). Nếu bỏ qua bước này, telemetry sẽ bị lỗi cross-org/global (lộ dữ liệu giữa các tổ chức).


- **Agent Performance:** Sử dụng endpoint `/api/v1/analytics/agents?project_id=...` đã được định nghĩa trong `AnalyticsDashboardHandler.AgentPerformance` để lấy các thông tin số task chạy, success rate, retry rate, tổng token và chi phí. (Cũng cần đảm bảo hàm này lọc theo `org_id` hợp lệ).
- **Audit Logs:** Sử dụng handler `AuditHandler` đã có để hiển thị lịch sử thao tác của tổ chức.


## 5. Bãi bỏ & Tránh phát sinh dư thừa (Cleanup & Bloat Prevention)

Để tối ưu hóa tài nguyên và giữ mã nguồn sạch sẽ, kế hoạch này nghiêm cấm việc phát sinh các phần dư thừa sau:
1. **Không tạo bảng `gateway_telemetry`**: Bãi bỏ hoàn toàn các đề xuất tạo bảng CSDL mới này. Thay vào đó, tái sử dụng và mở rộng bảng `token_usage` thông qua migration `000016`.
2. **Không tạo endpoint `/gateway-usage`**: Không phát sinh endpoint trùng lặp này. Thay thế hoàn toàn bằng việc mở rộng và gọi endpoint `/api/v1/analytics/token-usage`.
3. **Không tạo cột mới cho lỗi Workflow**: Bãi bỏ đề xuất thêm cột lưu lỗi của workflow vào `workflow_jobs`. Tận dụng 100% cột `last_error` sẵn có trong DB và mã nguồn hiện tại của Orchestrator.

## Tóm tắt Tác Động
- **Lợi ích**: Tối giản hóa thiết kế, không phát sinh bảng rác hay endpoint trùng lặp. Giúp lập trình viên dễ bảo trì hơn vì tận dụng 100% cơ chế ghi nhận usage hiện tại.


