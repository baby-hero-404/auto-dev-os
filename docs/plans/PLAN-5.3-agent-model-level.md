# PLAN 5.3: Agent & Workflow Model Level Integration

**Status:** Proposed  
**Feature Owner:** `docs/features/5.3-agent-system.md`, `docs/features/5.7-workflow-engine.md`

## 1. Ngữ Cảnh & Mục Tiêu
Sau khi Gateway được nâng cấp để hỗ trợ tra cứu tự động danh sách model theo các cấp độ **Fast / Balanced / Powerful** (như trong Kế hoạch 5.1), hệ thống Agent và Workflow Engine cần phải khớp cấu trúc dữ liệu với thiết kế này.
Hiện tại, Agent model dùng cột `model_route string` có giá trị default là `"balanced"`. Cần chuẩn hoá các validator và logic cập nhật Agent để nghiêm ngặt tuân theo 3 cấp độ (Level Groups) Fast, Balanced và Powerful.


## 2. Chuẩn hoá Agent Model và Cập nhật Validation (`server/internal/service/agent.go`)

Dù không đổi tên field trong database để tương thích ngược (`model_route`), nhưng ở tầng logic (Service & Validation), chúng ta sẽ ánh xạ nó 1:1 với khái niệm `Model Level Group` và thực hiện kiểm tra nghiêm ngặt thông qua các hàm validate thủ công có sẵn trong Service thay vì validator tags:

**1. Thêm hàm Helper Validate trong `server/internal/service/agent.go`:**
```go
func validateAgentModelRoute(route string) error {
	switch route {
	case "fast", "balanced", "powerful":
		return nil
	default:
		return ErrValidation("model_route must be fast, balanced, or powerful")
	}
}
```

**2. Tích hợp validation vào luồng Create:**
Trong `prepareCreateInput(ctx, input)`:
- Thực hiện `TrimSpace` biến `input.ModelRoute`. Nếu rỗng (`""`), tự động gán giá trị mặc định dựa theo Role bằng cách gọi `getDefaultModelRouteForRole(input.Role)`.
- Gọi `validateAgentModelRoute(input.ModelRoute)` và trả về lỗi nếu không hợp lệ.

**3. Tích hợp validation vào luồng Update:**
Trong `Update(ctx, id, input)`:
- Rất quan trọng: Chỉ xử lý nếu `input.ModelRoute != nil` để tránh lỗi nil pointer. Nếu chuỗi sau khi Trim là rỗng (`""`), bắt buộc phải dùng hàm repository nội bộ để `Get` lại current Agent lên (nhằm lấy được Role hiện tại của Agent đó) và gán mặc định bằng hàm `getDefaultModelRouteForRole(currentAgent.Role)`. Không được mù quáng gán cứng `"balanced"` để tránh làm hỏng mapping đã đề ra.
- Bắt buộc gọi `validateAgentModelRoute(*input.ModelRoute)` trước khi thực hiện update. Code hiện tại mới chỉ chuẩn hóa chuỗi rỗng chứ chưa chặn các giá trị rác như `gpt-4o`, do đó nếu quên validate ở bước này, hệ thống sẽ bị lỗi cấu hình thông qua PATCH request.


## 3. Cập nhật Orchestrator (`server/internal/orchestrator/`)

**[Baseline Verified]:** 
Kiểm tra codebase hiện tại cho thấy, `Orchestrator` trong `server/internal/orchestrator/orchestrator.go:1144` đã thực hiện việc đọc `Agent.ModelRoute` (đã được chuẩn hóa thành `fast`, `balanced`, hoặc `powerful`) và đưa vào tham số `RouteName` của `llm.RouteOptions` khi spawn Agent Task. Do đó, phần này đã hoàn thành ở baseline và không cần phát triển thêm.

**Chi tiết thiết lập đã được xác minh:**
```go
opts := llm.RouteOptions{
    OrgID:       org.ID,
    RouteName:   agent.ModelRoute, // Đã truyền 'fast', 'balanced', hoặc 'powerful' từ DB
    Complexity:  task.Complexity,  // Chỉ dùng làm fallback nếu RouteName trống
    // ...
}
```

## 4. Tự động đề xuất Level theo Agent Role

**Tại `server/internal/service/agent.go` - Hard-coded Role Mapping:**
Do schema DB hiện tại của `role_templates` chưa có cột `default_model_route`, thay vì tạo migration mới rườm rà, chúng ta sẽ thực hiện hard-coded mapping logic này ngay tại tầng Service. Bổ sung một helper function `getDefaultModelRouteForRole(role string) string`:
- **Role Planner, DB Architect**: Bắt buộc dùng cấp độ logic cao -> Gán `"powerful"`.
- **Role Backend, Frontend**: Tác vụ code thông thường -> Gán `"balanced"`.
- **Role Reviewer**: Tùy task, nhưng default là `"fast"`.
- **Role Security Auditor**: Đòi hỏi review sâu -> Gán `"powerful"`.

**Luồng thực thi (Service logic):**
Khi user tạo Agent mới và chọn Role từ Frontend, nếu Client không gửi `model_route` (hoặc rỗng), hàm `prepareCreateInput` sẽ tự động gọi `getDefaultModelRouteForRole(input.Role)` để gán level thay vì luôn luôn dùng mặc định `"balanced"`.


## 5. Cập nhật Dashboard (Chuẩn bị cho 5.9)

Dashboard hiện tại hiển thị Agent Status, nhưng cần làm rõ "Agent này đang hoạt động ở Level nào".
Tại API `GET /api/v1/agents` hoặc `GET /api/v1/projects/:id/agents`:
- Đảm bảo trả về trường `model_route` trực tiếp (không dùng alias `model_level`) để UI hiển thị Badge (ví dụ: Badge Đỏ "Powerful", Badge Xanh "Balanced"). 


## 6. Bãi bỏ & Dọn dẹp các Cấu hình model cứng

Để thống nhất cơ chế định tuyến qua Model Level Group, cần dọn dẹp các cấu hình cũ:
1. **Bãi bỏ các Input/Field chọn Model tĩnh**: 
   - Loại bỏ hoàn toàn các dropdown chứa tên model tĩnh (VD: `gpt-4`, `claude-3-5-sonnet`) trên giao diện cấu hình Agent.
   - Loại bỏ các logic xử lý hoặc parse tên model thô trong controller của Agent nếu còn sót lại.
2. **Cập nhật dữ liệu mẫu (Seeding Data)**:
   - Cập nhật toàn bộ các file seed template (nếu có) hoặc mặc định khởi tạo Agent để chuyển đổi trường `model_route` từ tên model tĩnh sang một trong ba giá trị chuẩn: `fast`, `balanced`, hoặc `powerful`.

## Tóm tắt tác động
- **Rủi ro rớt luồng**: Bằng 0, vì `ModelRoute` cũ vốn đã lưu `"balanced"`, tương thích 100% với định nghĩa Model Level Group mới.
- **Bảo trì**: Tránh việc developer tạo Agent với model name tĩnh (như `gpt-4o`) vì từ giờ Gateway mới là nơi quyết định `gpt-4o` thuộc Level nào.

