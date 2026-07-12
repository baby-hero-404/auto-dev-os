# 02. Rule System

**Status:** 🟡 In Progress (baseline implemented; hardening planned)  
**Owner docs:** `docs/features/product/02-rule-system.md` (this file)  
**Code areas:** `server/pkg/models/rule.go`, `server/internal/service/rule.go`, `server/internal/prompts/{builder,assembler,rules}.go` (prompt assembly), `server/internal/repository/rule.go`  
**Blocking decisions:** Final precedence model for global vs project rules and how conflicts are reported to users.  
**Acceptance criteria:** Global rules are immutable in agent system context, project rules are injected by task/project context, and conflicting local rules are rejected.

**Mục tiêu:** Kiểm soát hành vi của AI bằng một hệ thống quy tắc phân lớp. Quy tắc cấp cao (bảo mật, quản trị) không bao giờ bị ghi đè. Quy tắc cấp dưới (dự án, task) có thể tùy chỉnh nhưng không được phép xung đột với quy tắc cấp trên.

---

## Tại Sao Cần Rule System?

AI Agent có thể viết code rất nhanh nhưng không tự biết tuân thủ tiêu chuẩn của tổ chức. Rule System đảm bảo:

- **Mọi Agent** đều tuân thủ chính sách bảo mật chung (không lộ API key, luôn viết test...).
- **Mỗi dự án** có thể có coding conventions riêng (dùng Next.js, kiến trúc Hexagonal...) mà Agent phải tuân theo.
- **Mỗi task** có thể có ràng buộc cụ thể ("chỉ sửa file X, không đụng file Y") mà Agent phải tôn trọng.

---

## A. Kiến Trúc Phân Lớp (Strict Layered Context)

Quy tắc được chia thành 3 lớp, xếp theo mức ưu tiên từ cao xuống thấp:

*   **Enforcement Levels (Mức độ ràng buộc):**
    *   `strict` (Mặc định): Ràng buộc cứng. Agent bắt buộc tuân theo.
    *   `advisory`: Khuyến nghị/hướng dẫn. Có vai trò định hướng code style hoặc gợi ý tối ưu, Agent có thể linh hoạt điều chỉnh nếu ngữ cảnh yêu cầu.

### 1. Global Rules (Bất biến)

Các quy tắc cốt lõi về bảo mật và quản trị, áp dụng cho **tất cả Agent trong tổ chức**.

*   Được tiêm trực tiếp vào System Prompt — Agent không thể bỏ qua hay ghi đè.
*   **Ví dụ:** Không tiết lộ API key, luôn viết unit test, kiểm tra quyền truy cập, sử dụng parameterized queries.

### 2. Project Rules (Tùy chỉnh theo dự án)

Các quy ước mã hóa, kiến trúc, và style riêng cho từng dự án.

*   Được tiêm vào Task Context khi Agent xử lý task thuộc project đó.
*   **Ví dụ:** "Dùng Next.js App Router", "Tuân thủ kiến trúc Hexagonal", "Style bằng TailwindCSS".
*   **Xung đột:** Nếu Project Rule mâu thuẫn với Global Rule → hệ thống từ chối thực hiện và báo lỗi.

### 3. Task-specific Rules (Tạm thời, theo task)

Hướng dẫn và ràng buộc dành riêng cho một công việc cụ thể.

*   Được cung cấp bởi con người khi tạo task, hoặc bởi Agent Planner khi phân tích task.
*   **Ví dụ:** "Chỉ sửa logic trong file `payment.go`, không thay đổi file `migration.sql`".

## B. Cơ Chế Thực Thi

*   **Prompt Assembly:** Trước mỗi request đến LLM, Orchestrator tổng hợp quy tắc theo thứ tự: `Global Rules` → `Agent Role Constraints` → `Project Rules` → `Task Rules`. Quy tắc cấp cao được đặt trước, tạo nền tảng không thể ghi đè.
    *   **Agent Role Constraints (Ràng buộc vai trò):** Là các giới hạn và quy tắc ứng xử đặc thù gắn liền với vai trò của từng Agent (§04). Ví dụ: *Backend Specialist* chỉ được phép code logic server, *Reviewer Agent* chỉ được phép đánh giá/nhận xét lỗi mà không được tự ý sửa code, *QA Engineer* chỉ tập trung viết và chạy test. Việc chèn lớp ràng buộc này giúp AI luôn hoạt động đúng chức trách, không làm thay việc của vai trò khác.
*   **Conflict Detection:** Reviewer Agent kiểm tra code có vi phạm quy tắc hay không. Nếu vi phạm → từ chối duyệt PR và yêu cầu sửa.

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| Cursor Rules | Cách tổ chức rule system theo lớp cho AI coding assistant |
| OpenHands | Smart masking ngăn rò rỉ secret trong agent execution |
