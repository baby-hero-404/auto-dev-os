# PLAN UI: Frontend Gateway & Agent Updates

**Status:** Proposed  
**Feature Owner:** `web/` Frontend Application

## 1. Ngữ Cảnh & Mục Tiêu
Song song với việc nâng cấp Backend (Gateway Model Management, Agent Model Level, Telemetry), Frontend của hệ thống (thư mục `web/`) cũng cần được cập nhật để cho phép người dùng cấu hình và khai thác các tính năng mới này.
Kế hoạch này tập trung vào 3 màn hình chính: **AI Providers Settings**, **Agent Configuration**, và **Dashboard Analytics**.

## 2. Nâng cấp: Màn hình AI Providers Settings
**Đường dẫn dự kiến:** `/ai-providers` (hoặc `/settings/ai-providers` nếu hệ thống có chuyển hướng routing, cần đảm bảo cập nhật Navigation Menu để dẫn đúng về thư mục `web/src/app/ai-providers/page.tsx`).

**Yêu cầu tính năng:**
- **Danh sách Model theo Level Group:** Khi người dùng click vào một Provider đã kết nối (VD: OpenAI), bên dưới phần quản lý Multi-key, hiển thị 3 bảng/danh sách tương ứng với 3 Level:
  - ⚡ **Fast Models**
  - ⚖️ **Balanced Models**
  - 🚀 **Powerful Models**
- **Thao tác (CRUD):** 
  - Nút **"Add Model"** ở mỗi nhóm: Mở modal cho phép nhập Tên Model (VD: `gpt-4o-mini`) và thiết lập độ ưu tiên (Priority).
  - Nút **"Toggle Active"**: Bật/tắt nhanh một model (hữu ích khi model đó của vendor đang bị sập).
  - Nút **"Xóa"** model.
  - Hỗ trợ kéo thả (Drag & Drop) để thay đổi Priority (nếu có thể) hoặc nhập số trực tiếp.
- **Auto-Seeding UX:** Khi add provider lần đầu, UI tự động gọi API fetch lại danh sách để render ngay các model mặc định mà backend vừa auto-seed.

## 3. Nâng cấp: Màn hình Agent Configuration
**Đường dẫn dự kiến:** `/projects/:id/agents/new` hoặc `/settings/agents`

**Yêu cầu tính năng:**
- **Thay thế Field `Model` tự do:** Gỡ bỏ input text tự do nhập tên model (như `gpt-4`). Thay vào đó là một dropdown (Select Box) mang tên **"Model Intelligence Level"**.
- **Các Options trong Dropdown:**
  - ⚡ Fast (Tốc độ cao, chi phí rẻ)
  - ⚖️ Balanced (Mặc định - Cân bằng)
  - 🚀 Powerful (Tư duy kiến trúc sâu)
- **Auto-Select theo Role:** Khi người dùng chọn Agent Role (VD: chọn `Architect`), dropdown Model Level tự động nhảy sang `Powerful`. Chọn `Frontend Coder` -> nhảy sang `Balanced`.

- **Badges:** Hiển thị màu sắc trực quan cho các level này tại các bảng danh sách Agent.

## 4. Nâng cấp: Màn hình Dashboard & Analytics
**Đường dẫn dự kiến:** `/projects/:id/dashboard`

**Yêu cầu tính năng:**
- **Khu vực "Recent Failures" (Cực kỳ quan trọng):** 
  - Trong bảng hiển thị các task thất bại, thêm cột **"Failure Reason"**.
  - Map dữ liệu từ `workflow_jobs.last_error` của backend xuống.
  - Rút gọn lỗi (Truncate) nếu text quá dài và có nút "Xem chi tiết" (View Log) mở ra một Modal hoặc Side-panel chứa toàn bộ stack trace/raw error.
- **Khu vực "Gateway Usage" / "Cost Metrics":**
  - Hiển thị biểu đồ (Bar chart/Line chart) tổng Token và Chi phí (Cost) theo ngày/tuần.
  - **Phân rã theo Key (Key-level Analytics):** Nhờ việc Backend trả về `KeyLabel`, UI cần hiển thị Breakdown Chart (ví dụ: Pie chart hoặc Stacked Bar chart) cho thấy Key nào đang gánh nhiều tải nhất, Key nào có tỷ lệ lỗi (failure rate) cao nhất. Phân tích này giúp admin chủ động nạp thêm tiền hoặc thay key hỏng.
  - Có dropdown filter theo thời gian (7 ngày, 30 ngày, tháng này).

## 5. Bãi bỏ & Dọn dẹp Frontend (Cleanup & Deprecation)

Để tránh nhầm lẫn cho người dùng và làm sạch giao diện:
1. **Xóa bỏ các màn hình quản lý `Model Routes` cũ**:
   - Gỡ bỏ và xóa hoàn toàn các route, trang cài đặt hoặc tab cấu hình liên quan đến `model_routes` cũ (đã bị backend khai tử).
2. **Loại bỏ nhập tên Model tự do**:
   - Gỡ bỏ hoàn toàn ô nhập text tự do hoặc dropdown chứa danh sách tên model thô trong phần cấu hình của Agent.
3. **Cập nhật API Client & Types (`web/src/lib/api/gateway.ts`)**:
   - Xóa bỏ object API client cũ `modelRoutes` gọi đến endpoint `/organizations/{orgID}/model-routes`.
   - Tạo mới types `ProviderModel` và object API client `providerModels` gọi đến CRUD endpoint mới `/organizations/{orgID}/provider-models` (Lưu ý: bỏ tiền tố `/api/v1` vì base URL config của Axios/Fetch ở lib đã tự động chèn, nếu ghép thêm sẽ gây lỗi URL `/api/v1/api/v1/...`).
   - Tìm và thay thế tất cả các import cũ bằng client mới để Frontend compile thành công.


## 6. Tiêu chuẩn Xử lý Lỗi & UX (UX & Error Handling Standards)
- **Validation Form Model:** Ngăn chặn người dùng nhập sai format `level_group`. Frontend chỉ cho phép chọn từ danh sách dropdown cố định (fast, balanced, powerful). Bắt lỗi 400 Bad Request từ Backend và hiển thị Toast/Alert rõ ràng nếu người dùng cố tình gửi sai.
- **Trạng thái rỗng (Empty States):** Nếu một Provider chưa được map model nào cho một nhóm (ví dụ nhóm `Fast` trống), hiển thị một Empty State thân thiện ("No fast models configured yet. Add one to enable high-speed tasks.") thay vì một bảng trống vô hồn.
- **Xử lý giá trị Mặc định (`IsActive`):** Khi gọi API `POST`, frontend có thể không cần gửi trường `is_active` nếu user không tick chọn, Backend sẽ tự động gán là `true`. Frontend cần đồng bộ trạng thái Toggle trên UI để phản ánh đúng giá trị này.

## Tóm tắt Tiến trình (Phasing)
1. **Giai đoạn 1:** Cập nhật **AI Providers Settings** với UI phân tầng 3 level và hỗ trợ Drag-Drop Priority.
2. **Giai đoạn 2:** Áp dụng dropdown **Agent Configuration** thay cho input tự do và dọn dẹp các UI cũ (Model Routes).
3. **Giai đoạn 3:** Xây dựng các components biểu đồ cho **Dashboard Analytics** có hỗ trợ phân tích sâu tới cấp độ `KeyLabel`.

