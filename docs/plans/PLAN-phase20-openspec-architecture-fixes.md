# PLAN: Phase 20 - OpenSpec & Task System Architecture Hardening

## Objective
Thực thi các giải pháp kiến trúc để biến **OpenSpec thành Execution Contract (Nguồn chân lý duy nhất)**, khắc phục tình trạng Semantic Metadata Mixing, và củng cố sự ổn định của Orchestrator workflow theo báo cáo `docs/reports/log_task_report.md`.

---

## Phase 20.1: Triage & Cấp Cứu (P0 - ĐÃ HOÀN THÀNH)
*Lưu ý: Các hạng mục này đã được implement và verify trong session vừa qua.*

1. **Fail-Fast tại Analyze Step**
   - **Tình trạng cũ:** Analyze lỗi -> fallback về `Easy` -> đi tiếp sang Coding.
   - **Giải quyết:** Gỡ bỏ silent fallback trong `runAnalyzeProcess` (`server/internal/orchestrator/steps/analyze.go`). Trả về hard error và block workflow nếu output của LLM không parse được thành JSON hợp lệ.

2. **Khắc phục Semantic Metadata Mixing (`zentao-tool` bug)**
   - **Tình trạng cũ:** Agent đoán bừa tên repo từ Task Title (`change_name`), dẫn đến sinh ra đường dẫn giả (VD: `zentao-tool/main.go`).
   - **Giải quyết:** Sửa đổi logic nạp file trong `code_backend.go` và `code_frontend.go`. Ép LLM dùng đường dẫn tương đối gốc của Workspace, không cho phép tự suy diễn thư mục.

3. **OpenSpec trở thành Nguồn chân lý duy nhất (Single Source of Truth)**
   - **Tình trạng cũ:** Agent Coding đọc trộn lẫn `Task.Description` (mutable) và OpenSpec.
   - **Giải quyết:** Tại `server/internal/prompts/assembler.go`, tự động **cắt bỏ hoàn toàn** `Task.Description` nguyên thủy ra khỏi Prompt của Coder Agent nếu `OpenSpec` đã được tạo. Ép LLM Coding chỉ làm việc dựa trên Spec.

4. **Khai sinh Execution Manifest (Machine-readable Contract)**
   - **Tình trạng cũ:** Agent Coding phải parse Markdown text.
   - **Giải quyết:** Bổ sung việc inject nguyên khối JSON (chứa `affected_files`, `execution_plan`, `tasks`, `risks`) vào Prompt của Coder để thay thế việc phụ thuộc hoàn toàn vào văn bản Markdown.

5. **Freeze OpenSpec & Tăng cường Audit Logging**
   - **Tình trạng cũ:** Thiếu vết (trail) của Spec version, thiếu correlation ID cho log.
   - **Giải quyết:** 
     - Ghi nhận `SHA256 Spec Hash` ngay khi OpenSpec được duyệt (`AnalyzeStep`).
     - Bổ sung `step_id` và `attempt_id` vào `context` của worker, giúp cấu trúc hoá các log (Structured Logging).

---

## Phase 20.2: Hoàn thiện Validation & Semantic Task Model (P1)
*Trọng tâm: Đảm bảo Data Schema chặt chẽ và không cho phép lọt rác qua cửa ải Planner.*

### 1. Bổ sung Machine-readable Acceptance Criteria
- **File:** `server/pkg/models/task.go`
- **Action:** Thêm trường `AcceptanceCriteria []map[string]any` vào struct `TaskAnalysis`.
- **Lý do:** Giúp Reviewer và Tester có bộ tiêu chí chấm điểm rõ ràng (ví dụ: `[{"endpoint": "/api/users", "expected_status": 201}]`), loại bỏ đánh giá cảm tính.

### 2. Thiết lập Execution Boundaries (Giới hạn vùng an toàn)
- **File:** `server/pkg/models/task.go`
- **Action:** Thêm trường `ExecutionBoundaries map[string][]string` (gồm `allowed` và `forbidden` paths).
- **Lý do:** Định hướng rõ cho Code Agent không được phép sửa lan sang các thư mục cấm (VD: cấm sửa file `.github/workflows` nếu chỉ là task Frontend).

### 3. Sửa System Prompt của Planner Agent
- **File:** `server/internal/prompts/steps/analyze.md`
- **Action:** Update JSON schema requirement để bắt LLM phải xuất ra `acceptance_criteria` và `execution_boundaries` cùng lúc với OpenSpec Markdown.

### 4. Contract Validation Chặt Chẽ
- **File:** `server/internal/orchestrator/steps/analyze.go`
- **Action:** Thêm một function `ValidateExecutionManifest()`. Trước khi AnalyzeStep trả về Success, phải verify:
  - Spec có đủ file không?
  - YAML có parse được không?
  - Acceptance Criteria có empty không?
  - Nếu thiếu -> Force Retry LLM Planner.

---

## Phase 20.3: Tối ưu Context, Dependency & Resume (P2)
*Trọng tâm: Tinh gọn context đưa cho các Agent và xử lý triệt để Resume an toàn.*

### 1. Role-specific Context Separation
- **File:** `server/internal/prompts/assembler.go`
- **Action:** 
  - **Planner:** Nhận Architecture + User Requirement.
  - **Coder:** Chỉ nhận Execution Manifest + Affected Files Context.
  - **Reviewer:** Chỉ nhận Diff + Acceptance Criteria.
- **Lý do:** Ngăn chặn hiện tượng tràn Context Window (Context Bloat), giúp LLM focus tuyệt đối vào nhiệm vụ hiện tại.

### 2. Tái cấu trúc Workflow Resume
- **File:** `server/internal/orchestrator/worker.go`
- **Action:** Khi worker resume một task bị pause hoặc failed, phải load lại chính xác Spec Hash cũ. Nếu Spec Hash khác nhau, force Coder phải rollback code và chạy lại từ đầu.

### 20.6 Semantic Task Schema Updates (Next Phase Pre-Requisite)
- [x] Analyze JSON Strict Enforcement: Enforce `complexity`, `primary_category`, `execution_plan` alongside execution contracts.
- [x] Explicit Coder System Prompt update to demand `ExecutionBoundaries` obedience.
- [x] Trace and resolve Edge cases around `TaskComplexity` fallbacks.
- [ ] Migrate `Task.Description` to structured `Objective`, `Scope`, `Constraints` in Database. (Postponed for next cycle)

---

## Trình tự Triển Khai (Action Items)
1. **[X]** Bổ sung schema `AcceptanceCriteria` và `ExecutionBoundaries` vào `models/task.go`.
2. **[X]** Update System Prompt `analyze.md`.
3. **[X]** Viết hàm `ValidateExecutionManifest()` / Củng cố Logic Validator trong `AnalyzeStep`.
4. **[X]** Update Logic Role-specific Context trong `assembler.go` (Tách ngữ cảnh cho Planner, Coder, Reviewer).
5. **[X]** Tái cấu trúc Workflow Resume để sử dụng Spec Hash.
6. **[ ]** Semantic Task Update (Chuyển đổi Description thành Objective, Scope, Constraints).
