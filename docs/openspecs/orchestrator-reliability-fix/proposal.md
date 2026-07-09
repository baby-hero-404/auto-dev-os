# Proposal: Orchestrator & Prompt Reliability Fix

> **Evidence Source:** Real task execution trace `72d0ff65-c147-4271-96f2-481f0cf3db3e` (Zentao auto tool) — 13 LLM calls, task status: `failed`.
> **Cross-Reference:** `unified_prompt_architecture_report.md`, `prompt_construction_report_v2.md`, `prompt_contruction_report.md`

## Why

Tracing một task thực tế (Zentao auto tool — Hard complexity, 6 execution units) cho thấy hệ thống **thất bại ở giai đoạn Coding** sau khi Analyze/Plan thành công. Nguyên nhân gốc rễ gồm: Agent bị kẹt trạng thái (stale agent lock), thiếu cơ chế fallback khi credential hết hạn, và prompt không cung cấp đủ ngữ cảnh cho các coding step song song. Đây là các vấn đề blocking khiến hệ thống **không thể hoàn thành bất kỳ task Hard nào** trong thực tế.

## What Changes

### ISSUE-1: Agent Resource Leak — Stale Lock trên Agent Status
- Agent Backend bị kẹt trạng thái `assigned`/`running` sau khi step thất bại hoặc server restart.
- Workflow retry liên tục thất bại (`no available agent with role backend`) vì agent duy nhất bị khóa cứng.
- **Evidence:** Timeline cho thấy `code_backend_2` thất bại **10 lần liên tiếp** với cùng lỗi agent lock.
- **Fix đã áp dụng:** Thêm `ResetAllStatuses()` khi server khởi động. **Cần thêm:** Release agent trong `defer` của mỗi step thất bại.

### ISSUE-2: AI Gateway Credential Exhaustion — Không có Fallback Path
- 3 lần đầu `code_backend_2` thất bại với: `ai gateway exhausted routes: gemini/gemini-2.5-flash: no credentials`.
- Credential bị đánh dấu cooldown nhưng không có cơ chế retry sau cooldown.
- **Impact:** Khi credential duy nhất bị cooldown, toàn bộ coding pipeline bị block.

### ISSUE-3: Parallel Backend Steps — Không chia sẻ File Context
- `code_backend_0`, `code_backend_1`, `code_backend_2` chạy song song nhưng **mỗi prompt đều nhận cùng một Repository Structure rỗng** (`=== Repository Structure === .git:`).
- Không có cơ chế truyền output của `code_backend_0` (đã tạo `go.mod`, `main.go`, `config/`) vào prompt của `code_backend_1`/`code_backend_2`.
- **Evidence:** `code_backend_1` (call-008) output `models/commit.go` nhưng không biết `code_backend_0` đã tạo folder structure nào. LLM phải đoán và tạo lại `go.mod` từ đầu → xung đột file.

### ISSUE-4: Prompt Token Bloat — Full Execution Manifest trong mọi Coding Step
- Mỗi coding step nhận **toàn bộ Execution Manifest** (~200 dòng JSON) bao gồm acceptance criteria, risk domains, 6 execution phases, execution boundaries — dù step chỉ cần biết subtask của mình.
- **Evidence:** `code_backend_0` prompt có 4256 tokens, trong đó ~2000 tokens là Manifest không liên quan.
- Giảm token lãng phí = giảm cost + giảm nhiễu cho LLM.

### ISSUE-5: Missing Plan Step Output — `plan` bị Skip nhưng vẫn Cần Subtask Routing
- Plan step hoàn thành gần như ngay lập tức (`16:21:29 → 16:21:30`, 1 giây) mà không sinh output có ý nghĩa.
- Subtask routing (phân task cho `code_backend_0/1/2`) dựa vào `execution_units` từ Analyze output thay vì Plan output → Plan step hiện tại là **dead step** (bước chết), không đóng góp giá trị.

### ISSUE-6: Retry Storm — Vòng lặp vô hạn khi Step thất bại
- Khi `code_backend_2` thất bại, workflow retry từ checkpoint nhưng **không tăng backoff delay**.
- Timeline cho thấy 3 lần retry liên tiếp cách nhau chỉ 2 giây (`16:30:03` → `16:30:05` → `16:30:08`) → tạo "retry storm".
- Sau đó tiếp tục retry 7 lần nữa, mỗi lần đều chạy lại `code_backend_1` song song với `code_backend_2` → lãng phí LLM tokens cho `code_backend_1` chạy lại lần thứ 4, 5, 6, 7.

### ISSUE-7: LLM Path Confusion — Backend Agent tạo file ở đường dẫn sai
- `call-011` (code_backend_1): Output `tool_zentao/go.mod`, `tool_zentao/internal/model/...` — thêm prefix `tool_zentao/` thừa.
- `call-012` (code_backend_1 retry): Output `go.mod`, `internal/model/...` — không có prefix.
- `call-013` (code_backend_1 retry): Output `zentao-auto-tool/go.mod` — thêm prefix `zentao-auto-tool/` khác.
- **Root cause:** Prompt chỉ nói `Repository Structure === .git:` mà không cho LLM biết workspace root path thực sự là gì. Execution boundary root là `.` nhưng LLM không biết `.` đại diện cho thư mục nào.

## Capabilities

### New Capabilities
- `agent-auto-recovery`: Tự động phát hiện và giải phóng agent bị kẹt trạng thái sau timeout.
- `coding-step-context-sharing`: Truyền danh sách file đã tạo/sửa từ step trước vào prompt của step sau khi chạy song song.
- `prompt-pruning`: Loại bỏ các section không liên quan (full manifest, risk domains) khỏi coding step prompts.
- `retry-backoff`: Exponential backoff cho workflow-level retry.

### Modified Capabilities
- `code_backend step`: Cần nhận workspace file listing thực tế (từ git hoặc filesystem scan) thay vì static `.git:`.
- `plan step`: Cần thực sự phân tích execution_units và sinh subtask assignments có ý nghĩa, hoặc bị loại bỏ nếu Analyze đã làm đủ.
- `credential-pool`: Cần cơ chế wait-and-retry thay vì fail-fast khi tất cả credential đều trong cooldown.

## Impact

| Area | Files Affected |
|------|---------------|
| Agent Lifecycle | `orchestrator/agent_manager.go`, `orchestrator/worker.go`, `orchestrator/steps/code_backend.go`, `orchestrator/steps/code_frontend.go` |
| Prompt Assembly | `internal/prompts/assembler.go`, `orchestrator/llm_step.go` |
| Credential Pool | `internal/gateway/ai_gateway.go`, `internal/service/credential_pool.go` |
| Workflow Engine | `internal/workflow/engine.go`, `orchestrator/queue.go` |
| Step Registry | `orchestrator/step_registry.go` |
