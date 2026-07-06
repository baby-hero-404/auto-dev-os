# Báo cáo Phân tích Chi tiết: Tầng Orchestrator (Luồng Điều Phối)

**Đường dẫn:** `server/internal/orchestrator/`
**Mục tiêu:** Cung cấp cái nhìn sâu sắc và chi tiết nhất về cách Orchestrator hoạt động, các luồng thực thi (flow), và vai trò của từng file/hàm chính. Báo cáo này giúp đưa ra quyết định khi muốn cải tiến hoặc refactor (ví dụ: song song hóa, thay đổi patch engine, tối ưu context).

---

## 1. Tổng quan Kiến trúc (Architecture Overview)

Package `orchestrator` là trái tim của Auto Code OS. Nó đứng giữa Backend Database (Task, Project) và các tác nhân bên ngoài (LLM, Docker Sandbox, Git). Nhiệm vụ của nó là:
- **Tiếp nhận Task:** Chuyển đổi trạng thái từ Todo -> In Progress.
- **Quản lý Vòng đời (Lifecycle):** Cấp phát AI Agent, chuẩn bị Git Workspace, gắn Context.
- **Thực thi DAG (Directed Acyclic Graph):** Chạy quy trình theo từng bước (Step), tạm dừng, khôi phục từ Checkpoint.
- **Giao tiếp LLM & Sandbox:** Không trực tiếp gọi LLM hay Bash mà thông qua các Adapter và Sub-packages.
- **Đóng gói & Dọn dẹp:** Giải phóng Agent, xoá/lưu trữ Workspace.

---

## 2. Luồng Thực Thi Chính (End-to-End Flow)

Một Task chạy qua Orchestrator sẽ đi theo vòng đời sau:

1. **Trigger (Khởi tạo):** API Handler gọi `orchestrator.Execute(taskID)`. Hệ thống tạo ra một `WorkflowJob` trong DB (hoặc resume nếu đang paused).
2. **Worker Polling (Lấy Job):** Goroutine trong `worker.go` (`StartWorker`) liên tục quét DB để lấy các job đang đợi.
3. **Chuẩn bị (Preparation):**
   - Lấy thông tin Task & Job.
   - Gọi `AgentAssigner` để mượn (Assign) một Agent rảnh.
   - Gọi `wkspace.Manager.EnsureWorkspaceCloned()` để clone git repo xuống host và mount vào Sandbox container.
4. **Khởi tạo Workflow Engine:**
   - Dựa vào `task.Complexity` (Dễ, Trung bình, Khó), hệ thống nạp một DAG tương ứng (`EasyWorkflow`, `MediumWorkflow`, `HardWorkflow` từ package `workflow`).
   - Đăng ký các hàm chạy từng bước qua `stepRunners()` (từ `step_registry.go`).
5. **Thực thi Bước (Step Execution):**
   - Engine chạy từng bước (VD: `ContextLoad` -> `Analyze` -> `Code`).
   - **Hook OnEvent:** Mỗi khi xong 1 bước, hệ thống gọi hàm `checkpoint()` để lưu output JSON vào DB và ghi log sự kiện.
   - Nếu gặp lỗi `ErrReviewFixLoop`: Engine tự động quay lại bước Review.
6. **Hoàn thành / Thất bại:**
   - Gọi Learning/Memory engine để phân tích kết quả, đề xuất update rule.
   - Giải phóng Agent (`agents.Release`).
   - Xoá lock workspace và lên lịch xoá workspace (`cleanupWorkspaceAfterFinalState`).

---

## 3. Chi tiết Các File Cốt Lõi và Hàm Chính

### 3.1. `orchestrator.go` (Cấu hình & Các lệnh API)
Đóng vai trò là entrypoint nhận yêu cầu từ HTTP Controller.

* **Struct chính:** `Orchestrator` (chứa các dependencies như `TaskRepository`, `WorkflowRepository`, `AgentAssigner`, `Sandbox`, `GitOps`, `LLM`, v.v.).
* **Hàm chính:**
  * `New(...)`: Constructor, inject các dependencies (Sử dụng Option pattern).
  * `Execute(ctx, taskID)`: Đưa task vào hàng đợi. Chuyển trạng thái task sang `ContextLoading`. Khởi tạo `WorkflowJob`.
  * `RetryFromLastStep(ctx, taskID)`: Chạy lại một task bị Failed từ checkpoint gần nhất.
  * `ApproveMerge(ctx, taskID)`: Được gọi khi con người bấm "Approve PR" trên Web UI. Hàm này gọi GitOps để merge PR và chuyển task status thành `Merged`.
  * `ClearCheckpointsForRepair(...)`: Xóa các checkpoint từ bước code trở đi để force Agent làm lại khi PR bị Reject.

### 3.2. `worker.go` (Vòng lặp Vô Tận & Xử lý Job)
Nơi chứa logic chạy ngầm thực sự. Tránh chặn HTTP thread.

* **Hàm chính:**
  * `StartWorker(ctx, interval, concurrency)`: Chạy vòng lặp polling. Quản lý số lượng luồng đồng thời bằng semaphore. Gọi `workflows.ClaimNext(ctx)` để lấy Job. Cứ mỗi job, nó spawn một goroutine chạy `run(jobID)`.
  * `run(ctx, jobID)`: **Trái tim của hệ thống thực thi**.
    1. Update job sang `Running`.
    2. Cấp phát Agent.
    3. Setup Workspace.
    4. Nạp Memory (tiền đề cho LLM).
    5. Cấu hình `workflow.Engine` và móc nối hook `OnEvent` (lưu checkpoint, ghi timeline log).
    6. Dựng DAG theo Complexity.
    7. Tải checkpoint cũ (nếu có) để resume.
    8. Gọi `engine.Run()` hoặc `engine.Resume()`.
    9. Nếu lỗi vòng lặp (ReviewFix Loop) -> tự động quay lại bước Review/Fix.
    10. Trong mọi trường hợp thoát (defer panic/success), luôn giải phóng Agent và lưu trạng thái cuối.

### 3.3. `step_registry.go` (Bản đồ các Bước)
Liên kết giữa Workflow Engine thuần túy (DAG) và logic nghiệp vụ.

* **Hàm chính:**
  * `stepRunners(task, agent, jobID, jobStep)`: Trả về một `map[string]workflow.StepFunc`.
  * Nó khởi tạo tất cả các Step cụ thể (nằm trong thư mục `steps/`) như `ContextLoadStep`, `AnalyzeStep`, `PlanStep`, `CodeBackendStep`, `TestStep`, v.v.
  * Nó bọc (wrap) từng step bằng `WithCheckpointRecovery` để đảm bảo nếu server sập giữa chừng, bước nào chạy xong rồi sẽ được bỏ qua.

### 3.4. `interfaces.go` (Dependency Inversion)
Định nghĩa các interface mà Orchestrator cần, giúp dễ mock khi test và tách biệt module.
* `AgentAssigner` (Cấp phát agent)
* `PromptBuilder` (Lắp ráp prompt)
* `GitOpsClient` (Clone, PR, Push)
* Các `Repository` (DB CRUD)

---

## 4. Chi tiết Các Sub-packages Trọng Yếu

Để code không bị phình to (monolith), `orchestrator` đẩy logic cụ thể xuống các thư mục con. Đây là các điểm bạn sẽ cần sửa nếu muốn thay đổi cách hoạt động:

### 4.1. `patch/applier.go` (Động cơ Patch - Cực kỳ quan trọng)
Đây là nơi LLM biến text trả về thành code thật sự trên ổ đĩa.
* **Flow:** LLM sinh ra output -> Trích xuất block patch dạng diff -> Gửi vào `ApplyPatch`.
* **Logic bên trong `ApplyPatch`:**
  1. Phân tích file nào bị đổi (`modifiedFiles`), kiểm tra xem có nằm trong danh sách cho phép của bước Analyze (`AffectedFiles`) không (Bảo mật).
  2. Dùng Sandbox chạy một bash script gọi `git apply` để chèn code vào workspace một cách an toàn.
  3. Hỗ trợ multi-repo (tách patch cho từng microservice khác nhau).
* **Điểm cần nâng cấp:** Chỗ này đang dựa thuần túy vào chuỗi (String/Git diff). Để giảm lỗi, cần thiết kế thêm AST Parser hoặc self-healing (Như Option A ở báo cáo Brainstorm).

### 4.2. `steps/` (Thực thi chi tiết của từng Node)
Mỗi file trong thư mục này đại diện cho 1 bước trong DAG (VD: `step_analyze.go`, `step_code.go`).
* Các file này thực hiện việc chuẩn bị Prompt, gọi hàm `RunLLMStep` (giao tiếp AI Gateway), parse JSON schema đầu ra, và cập nhật trạng thái (VD: Lưu subtasks để chia task nhỏ).

### 4.3. `wkspace/` và `repoutil/`
Quản lý File system.
* `wkspace`: Quản lý mount thư mục host vào docker, retention policy (giữ file 3 ngày để debug, sau đó xoá để tránh tràn ổ cứng).
* `repoutil`: Xử lý việc tính toán Diff (git diff) giữa nhánh gốc và code mới do Agent viết ra để làm feedback nhồi vào cho bước `ReviewStep`.

### 4.4. `learning/` (Tự động Cải tiến)
Cơ chế tự động lấy output từ các vòng `Review` và `Fix` để tinh chỉnh (suggest) Prompt Rules mới cho các task tương tự trong tương lai.

---

## 5. Các Chỗ Cần Quan Tâm Nếu Muốn Điều Chỉnh (Quyết định)

1. **Nếu bạn muốn chạy song song Frontend và Backend:**
   * Cần mở file `server/internal/workflow/dag.go` (ngoài orchestrator) để cấu hình lại sơ đồ đồ thị có chứa node `Join`.
   * Cần sửa `worker.go` trong `orchestrator` để cho phép cấp phát 2 Agent cùng lúc cho 1 Task. Hiện tại, `AgentAssigner.Assign` trong `run()` chỉ cấp 1 Agent cho toàn bộ vòng đời.

2. **Nếu bạn muốn cải thiện độ chính xác của Code (Giảm lỗi Review Fix):**
   * Tập trung sửa `server/internal/orchestrator/patch/applier.go` để xử lý diff tốt hơn.
   * Cải tiến `server/internal/orchestrator/steps/step_code_backend.go` để nạp thêm file AST hoặc Error Log vào prompt trước khi bắt LLM viết patch.

3. **Nếu bạn muốn UI mượt hơn (Real-time Streaming):**
   * Trong `worker.go`, chỗ hàm callback `OnEvent(ctx, event)` của `engine := &workflow.Engine{...}`, thêm code bắn Message Broker (Redis/Kafka) hoặc SSE Hub ra ngoài Web UI mỗi khi `event.Status` thay đổi, thay vì chỉ ghi DB.

---
Báo cáo này cung cấp bản đồ đầy đủ để bạn tự tin duyệt mã và thực hiện những quyết định kiến trúc tiếp theo một cách an toàn và đúng đắn.
