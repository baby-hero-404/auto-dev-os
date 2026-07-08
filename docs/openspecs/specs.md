# Specs: Orchestrator & Prompt Reliability Fix

> **Evidence Source:** Real task trace `72d0ff65` — 13 LLM calls, 10 retry failures.

---

## ADDED Requirements

### Requirement: Agent Auto-Recovery (ISSUE-1)

Hệ thống **PHẢI** tự động phát hiện và giải phóng các Agent bị kẹt trạng thái (`assigned`/`running`) sau một khoảng thời gian timeout cấu hình được.

#### Scenario: Server Restart Recovery
- **WHEN** server API khởi động lại
- **THEN** hệ thống **PHẢI** reset tất cả agent có status `assigned` hoặc `running` về `idle`
- **STATUS:** ✅ Đã triển khai (`cmd/api/main.go` — `ResetAllStatuses()`)

#### Scenario: Step Failure Agent Release
- **WHEN** một coding step (`code_backend`, `code_frontend`) thất bại với bất kỳ lỗi nào
- **THEN** agent được gán cho step đó **PHẢI** được trả về trạng thái `idle` trong vòng 1 giây
- **STATUS:** ⚠️ Hiện tại chỉ có `defer` release ở top-level worker, nhưng `assignByRole` trong code_backend gán agent mới mà không release khi step fail trước khi return error.

#### Scenario: Agent Heartbeat Timeout
- **WHEN** một agent ở trạng thái `running` quá 30 phút mà không có checkpoint mới
- **THEN** hệ thống **PHẢI** tự động chuyển agent về `idle` và ghi audit log

---

### Requirement: Credential Pool Resilience (ISSUE-2)

AI Gateway **PHẢI** hỗ trợ cơ chế chờ và thử lại (wait-and-retry) khi tất cả credential của một provider đều đang trong trạng thái cooldown.

#### Scenario: All Credentials in Cooldown
- **WHEN** tất cả credential cho provider `gemini` đều đang cooldown
- **THEN** gateway **PHẢI** chờ đến khi credential có cooldown ngắn nhất hết hạn, sau đó tự động retry
- **THEN** gateway **KHÔNG ĐƯỢC** trả về lỗi `exhausted routes` ngay lập tức

#### Scenario: Credential Recovery Notification
- **WHEN** một credential thoát khỏi trạng thái cooldown
- **THEN** hệ thống **NÊN** ghi log level `info` thông báo credential đã sẵn sàng

---

### Requirement: Coding Step Context Sharing (ISSUE-3)

Khi nhiều coding steps chạy song song, output files từ các step đã hoàn thành **PHẢI** được truyền vào prompt của các step chưa hoàn thành.

#### Scenario: Parallel Backend Steps with Dependencies
- **WHEN** `code_backend_0` hoàn thành và tạo ra files `go.mod`, `main.go`, `config/config.go`
- **AND** `code_backend_1` chưa bắt đầu hoặc đang chờ
- **THEN** prompt của `code_backend_1` **PHẢI** bao gồm danh sách file đã tạo bởi `code_backend_0` trong section `### Workspace Affected Files ###`

#### Scenario: Repository Structure Injection
- **WHEN** coding step bất kỳ được thực thi
- **THEN** `=== Repository Structure ===` trong prompt **PHẢI** phản ánh trạng thái thực tế của filesystem tại thời điểm step bắt đầu, KHÔNG được là snapshot tĩnh từ context_load

---

### Requirement: Prompt Pruning for Coding Steps (ISSUE-4)

Prompt của coding steps **PHẢI** chỉ chứa thông tin cần thiết cho subtask được giao.

#### Scenario: Coding Step Prompt Content
- **WHEN** hệ thống tạo prompt cho `code_backend_N`
- **THEN** prompt **PHẢI** bao gồm:
  - System prompt + Global Rules
  - Task title + Assigned Subtask (chỉ subtask được giao, không phải toàn bộ plan)
  - Clarification answers (Q&A)
  - OpenSpec Design section (context, goals, decisions)
  - Repository structure hiện tại
  - Danh sách file affected bởi các step trước
- **THEN** prompt **KHÔNG ĐƯỢC** bao gồm:
  - Full Execution Manifest JSON (acceptance_criteria, risk_domains, execution_boundaries, execution_phases)
  - Risks/risk_details arrays
  - Các subtask của steps khác

---

### Requirement: Retry Backoff and Circuit Breaker (ISSUE-6)

Workflow retry **PHẢI** áp dụng exponential backoff và có giới hạn tối đa.

#### Scenario: Step Failure Retry
- **WHEN** một step thất bại và workflow cần retry
- **THEN** delay giữa các lần retry **PHẢI** tăng theo công thức: `min(2^attempt * base_delay, max_delay)` với `base_delay=2s`, `max_delay=60s`
- **THEN** sau `max_retries` lần (mặc định 3), workflow **PHẢI** dừng và đánh dấu task `failed`

#### Scenario: Parallel Step Re-execution Prevention
- **WHEN** `code_backend_2` thất bại và workflow retry
- **AND** `code_backend_1` đã có checkpoint `success`
- **THEN** workflow **KHÔNG ĐƯỢC** chạy lại `code_backend_1`
- **STATUS:** ⚠️ Hiện tại `code_backend_1` chạy lại mỗi lần retry (evidence: call-008, call-010, call-011, call-012, call-013 — total 5 lần chạy lại cùng step)

---

### Requirement: Workspace Path Clarity (ISSUE-7)

Prompt **PHẢI** cung cấp rõ ràng đường dẫn workspace root cho LLM.

#### Scenario: Single Repository Workspace
- **WHEN** workspace chỉ có 1 repository
- **THEN** prompt **PHẢI** bao gồm instruction: `Your workspace root is the repository root. All file paths in your diff MUST be relative to this root (e.g., --- a/internal/model/commit.go). Do NOT add any prefix like the repository name.`

#### Scenario: File Path Validation
- **WHEN** LLM output chứa file paths có prefix trùng với tên repository (e.g., `tool_zentao/go.mod`)
- **THEN** Patch Validator **NÊN** strip prefix tự động và ghi warning log

---

## MODIFIED Requirements

### Requirement: Plan Step Effectiveness (ISSUE-5)

Plan step **PHẢI** sinh ra output có giá trị thực sự hoặc bị bỏ qua khi Analyze đã cung cấp đủ thông tin.

#### Scenario: Analyze Already Provides Execution Units
- **WHEN** Analyze step output chứa `execution_units` với đầy đủ `dependencies`, `tasks`, và `execution_profile`
- **THEN** Plan step **NÊN** được skip (thời gian hiện tại: 1 giây → chỉ copy data)
- **OR** Plan step **PHẢI** refine subtask assignments dựa trên workspace state thực tế

---

## REMOVED Requirements

(Không có yêu cầu nào bị loại bỏ trong phase này)
