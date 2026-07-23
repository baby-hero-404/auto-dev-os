---
sources:
  - "server/**"
---

# 04. Semantic Boundaries & Filesystem RBAC

**Status:** 🟢 Implemented (audited 2026-07-12: `ExecutionBoundary`/`ExpandedBoundary` in `pkg/models/task.go`, capability validation in `patch/policy_engine.go`, JIT expansion + audit trail in `steps/boundary_tool_executor.go` — matches or exceeds this doc's design)
**Owner docs:** `docs/features/engineering/04-semantic-boundaries.md` (this file)
**Code areas:** `server/internal/orchestrator/patch/applier.go`, `server/pkg/models` (`ExecutionBoundary`)

**Mục tiêu:** Đặc tả cơ chế phân quyền hệ thống file (Filesystem RBAC) cho các AI Agent trong Auto Code OS, chuyển dịch từ mô hình "danh sách trắng tĩnh" (Static ACL) sang **Biên giới Ngữ nghĩa (Semantic Boundaries)**.
Hệ thống giải quyết triệt để lỗi "Security Violation" khi LLM tự động thực hiện Test-Driven Development (TDD) hoặc tái cấu trúc mã nguồn (Refactoring), đồng thời thiết lập cơ chế **Just-In-Time (JIT) Expansion** để mở rộng ranh giới an toàn một cách tự động và minh bạch.

---

## 1. Feature 1: Semantic Capabilities & RBAC

### 1.1 Vấn đề
Hệ thống trước đây sử dụng mô hình ACL (Access Control List) cứng nhắc bằng trường `affected_files`. Bước `Analyze` buộc phải đoán trước chính xác 100% danh sách các file mà bước `Code` sẽ sửa. Trong thực tế (như khi tách hàm helper, viết Unit Test, cài dependency), việc AI Agent bất ngờ tạo file mới nằm ngoài danh sách dự kiến sẽ ngay lập tức kích hoạt lỗi `Security Violation`, phá vỡ toàn bộ luồng TDD.

### 1.2 Thiết kế Cơ chế
- **Role-Based Access Control (RBAC):**
  - Thay thế danh sách file cứng thành một **ExecutionBoundary** bao gồm: `Module` (tên miền chức năng), `Root` (thư mục giới hạn - Blast Radius), và `Capabilities` (quyền hạn hành động).
- **Semantic Capabilities:**
  - Định nghĩa các "quyền" (permissions) thay vì "đối tượng" (files). Ví dụ:
    - `modify_existing`: Cho phép sửa các file đã tồn tại trong Root.
    - `create_helper`: Cho phép tạo các file bổ trợ (`utils.go`, `types.ts`) trong Root.
    - `create_test`: Cho phép tạo các file kiểm thử (`*_test.go`, `*.spec.ts`).
    - `modify_exports`: Cho phép sửa các file export module (`index.ts`, `mod.rs`).
    - `add_dependency`: Đặc quyền sửa file cấu hình root (`go.mod`, `package.json`).
- **Validation Engine:**
  - Lõi `Patcher` không kiểm tra file có nằm trong whitelist nữa, mà phân tích ý định thay đổi (Change Intent) của LLM. Ví dụ: Sửa `go.mod` sẽ kích hoạt check quyền `add_dependency`.

---

## 2. Feature 2: Just-In-Time (JIT) Expansion & Risk Evaluation

### 2.1 Vấn đề
Dù có bộ quyền Capabilities, hệ thống Patcher vẫn cần một tiêu chuẩn đánh giá rủi ro để quyết định xem hành động mở rộng ranh giới có an toàn hay không, và phải giải thích được cho Reviewer (con người) tại sao file đó lại được sinh ra mà không có trong kế hoạch Analyze ban đầu.

### 2.2 Thiết kế Cơ chế
- **Risk Evaluation Heuristics (Đánh giá Rủi ro):**
  - **LOW**: Việc tạo Helper, Test, Mock files nằm gọn trong `Root` sẽ được tự động chấp thuận.
  - **MEDIUM**: Sửa đổi config files (`go.mod`, `Cargo.toml`) được phép nhưng sinh cảnh báo Warn cho PR.
  - **HIGH & CRITICAL**: Xâm phạm Infrastructure (Dockerfile, `.github`) sẽ bị từ chối thẳng thừng để bảo vệ hệ thống cốt lõi.
- **Boundary Expansion Logging (Audit Trail):**
  - Khi Patcher kích hoạt JIT Expansion, thay vì âm thầm cấp quyền, nó sẽ sinh ra một Object `ExpandedBoundary` lưu vào Database:
    ```json
    { "file": "internal/repo/helper.go", "reason": "create_helper", "risk": "low" }
    ```
  - Review UI sẽ hiển thị Audit Trail này giúp con người dễ dàng hiểu rõ ngữ cảnh quyết định của hệ thống.

---

## 3. Lộ trình Triển khai (Implementation Phasing)

- **Phase 1: Update Schema & Prompts**
  - Thêm `ExecutionBoundary` vào `models.TaskAnalysis`.
  - Cập nhật prompt của `Analyze` để sinh ra JSON RBAC thay vì array chuỗi.
- **Phase 2: Refactor Validation Engine**
  - Viết lại hàm `MatchAffectedFile` và `IsUnderAffectedDir` thành bộ `SemanticValidator`.
  - Triển khai logic JIT Boundary Expansion trong `patch/applier.go`.
- **Phase 3: Integration & Logging**
  - Kết nối Patcher với Task State để ghi nhận `ExpandedBoundary` về Database.
  - Hiển thị bảng Audit Expansion Logs trên Web UI của Frontend.
