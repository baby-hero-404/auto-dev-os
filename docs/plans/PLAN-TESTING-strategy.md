# PLAN TESTING: Comprehensive Testing Strategy

**Status:** Proposed  
**Scope:** Backend (Go Unit/Integration Tests) & Frontend (Playwright E2E/Route-Mocking)

## 1. Ngữ Cảnh & Mục Tiêu
Cùng với sự thay đổi kiến trúc khổng lồ từ các PLAN 5.1 (Gateway), 5.3 (Agent), 5.9 (Telemetry) và UI, hệ thống các bài kiểm thử (Test Suites) hiện tại chắc chắn sẽ bị gãy (break) do gọi đến các interface cũ, các trường dữ liệu không còn tồn tại, hoặc thiếu hụt coverage cho các luồng validation mới.

Kế hoạch này cung cấp danh sách kiểm tra (checklist) chi tiết để **Dọn dẹp triệt để test cũ** và **Bổ sung các test case mới**, đảm bảo hệ thống đạt mức độ "chống chịu lỗi" (bullet-proof) trước khi merge code.

---

## 2. Dọn Dẹp (Deprecation): Xóa các Test Case Outdated

Việc giữ lại các bài test cũ gọi đến mã nguồn đã bị bãi bỏ sẽ gây lỗi Compile và nhiễu loạn luồng CI/CD.

### 2.1. Backend Tests
1. **Xóa Test của Model Routes Cũ:**
   - Xóa toàn bộ file test của `ModelRouteService` (ví dụ: `server/internal/service/model_route_test.go`).
   - Xóa file test của `ModelRouteRepository` (ví dụ: `server/internal/repository/model_route_test.go`).
   - Xóa file test của HTTP Handler tương ứng `ModelRouteHandler`.
2. **Sửa Test Khởi tạo Gateway (`main.go` / `gateway_test.go`):**
   - Loại bỏ các mock liên quan đến `ModelRouteService`.
   - Gỡ bỏ bài test kiểm tra cơ chế filter bằng credential trực tiếp (tiền lọc) bên trong hàm get combo entries, do tính năng này đã bị hủy bỏ (PLAN-5.1 Đợt 9).
3. **Sửa Test của Analytics:**
   - Xóa/Sửa các test kì vọng endpoint `/api/v1/analytics/token-usage` chỉ lọc theo `project_id`. Thiết kế mới đã ép buộc `org_id` (PLAN-5.9 Đợt 10).

### 2.2. Frontend Tests
1. **Xóa E2E/Component Test giao diện cũ:**
   - Xóa các test kiểm tra ô "Nhập tay tên model" (Free-text input) trong màn hình cấu hình Agent.
   - Xóa các test điều hướng (routing) trỏ tới các trang quản lý `/settings/model-routes` cũ.
2. **Xóa Mock API Client cũ:**
   - Gỡ bỏ mock data và test coverage liên quan đến object `modelRoutes` trong `gateway.ts`.

---

## 3. Bổ Sung (Addition): Xây dựng Test Case cho Logic Mới

Đây là cốt lõi để đảm bảo các thay đổi phức tạp ở service layer không bị hồi quy (regression) trong tương lai.

### 3.1. Gateway Model Management (Theo PLAN-5.1)
- **`ProviderModelService` Unit Tests:**
  - `Test_Create_ValidateLevelGroup`: Cố tình truyền `level_group` = "gpt-4o" và kỳ vọng Service ném lỗi HTTP 400 Bad Request.
  - `Test_Create_DefaultIsActive`: Truyền `IsActive = nil` từ input và assert repository nhận được giá trị `true`.
  - `Test_List_QueryFilter`: Giả lập HTTP GET với query param value viết hoa (VD: `provider=OPENAI`), assert Service tự động chuyển đổi thành viết thường (lowercase) và gọi DB chính xác.
- **`CredentialPoolService` Auto-seeding Test:**
  - `Test_CreateCredential_AutoSeed_Idempotency`: Gọi hàm tạo Credential 2 lần liên tiếp. Assert DB không bị văng lỗi Duplicate Key (nhờ `ON CONFLICT DO NOTHING`) và số lượng model seed không bị nhân đôi.
- **Gateway Runtime `ResolveModels` Test:**
  - `Test_Gateway_NoPrefilter_SideEffect`: Assert rằng hàm resolve model không hề gọi đến `SelectCredential` (không sinh ra giả mạo Audit log).
  - `Test_Gateway_EmptyDB_Fallback`: Xóa sạch bảng `provider_models`, gọi luồng Gateway và assert hệ thống gọi hàm `defaultEntries()` an toàn mà không panic.

### 3.2. Agent Model Level (Theo PLAN-5.3)
- **`AgentService` Validation Tests:**
  - `Test_UpdateAgent_ModelValidation`: Gọi `Update` với giá trị rác cho field `model_route`. Assert hệ thống ném lỗi trước khi lưu vào DB. (Bảo chứng cho Finding 54).
  - `Test_CreateAgent_HardcodedRoleMapping`: Khởi tạo Agent với `Role = "db-architect"` và `model_route = ""`. Assert Service tự động lấy hàm helper và lưu `powerful` xuống Database. Khởi tạo `Role = "frontend"` -> lưu `balanced`.

### 3.3. Observability & Telemetry (Theo PLAN-5.9)
- **Telemetry Async Hook Tests:**
  - `Test_Telemetry_ContextTimeout`: Đảm bảo goroutine sử dụng `context.WithoutCancel` kèm Timeout, không bị ngắt rữa chừng khi Request HTTP gốc đã Return.
  - `Test_Telemetry_RecordTier`: Assert biến `Tier` ("fast", "balanced"...) được trích xuất từ `lastEntry.Tier` và ghi nhận thành công vào `llm.UsageRecord`. Fallback = "balanced" nếu rỗng.
  - `Test_Telemetry_FailureKeepsCredentialID`: Giả lập request gửi lên LLM bị ném lỗi HTTP 401. Assert rằng `record.CredentialID` **vẫn được giữ nguyên** chứ không bị rỗng, nhằm thống kê failure rate theo key.
- **Analytics API & Repository Tests:**
  - `Test_Analytics_RequireOrgID`: Mock call GET `/token-usage` thiếu `org_id` context. Assert HTTP 403/400.
  - `Test_Analytics_JoinKeyLabel`: Mock data DB, gọi repo `AnalyticsRepo.TokenUsage`. Assert câu lệnh SQL gộp nhóm theo `credential_id` trả về đúng giá trị cột `pc.label AS key_label`.

### 3.4. Frontend UI/UX (Theo PLAN-UI)
- **Playwright E2E & Route-Mocking Tests:**
  - `Test_AgentForm_DropdownOnly`: Điều hướng đến màn hình Agent Form. Assert field Model Route là thẻ `<select>` chứa đúng 3 option cứng, không cho gõ text.
  - `Test_AIProvider_EmptyState`: Sử dụng Playwright API route mocking để trả về danh sách rỗng cho group "Fast". Assert giao diện render ra Empty State thân thiện ("No fast models configured yet...").
  - `Test_Dashboard_KeyLevelAnalytics`: Mock API trả về `TokenUsageSummary` chứa `KeyLabel`. Assert biểu đồ Breakdown (Pie/Stacked) hiển thị đúng các label (VD: "OpenAI-Prod-Key1").
  - `Test_Dashboard_FailureReason`: Mock API trả về `last_error` rất dài. Assert text được truncate trên giao diện và hiển thị đầy đủ log trong Modal khi click nút "View Log".

---

## 4. Tóm tắt Tác Động
Việc thiết lập Bộ Test Mới (New Test Suite) này là chốt chặn (gatekeeper) cuối cùng. Nó đảm bảo các dev ở Implementation Phase (Giai đoạn gõ code) sẽ tuân thủ tuyệt đối các design patterns đã chốt ở những phiên họp kiến trúc vừa qua (vd: Consumer-defined interface, Default Values, No Side-effects). Mọi Pull Request (PR) đều phải pass bộ Test này trước khi Merge.
