# Tasks: Execution Provider Routing — Adoption Gaps

## P0 — close the 3 gaps

### Task 1.1: `cliStepRunner` đi qua Router
- [x] `cli_spec_step.go`: thêm field `credID string`, sửa `resolveConfig` gọi `o.ResolveExecutionProvider`, trả lỗi nếu `resolved.Type != "cli"`.
- [x] `RunCLIStep`: thêm nhánh `res.QuotaExceeded && r.credID != "" && r.o.cooldownSetter != nil` → `SetCooldown`, đặt ngay sau `eng.RunCodeStep`, trước khi log/save artifact (không đổi thứ tự các bước còn lại).
- [x] `cli_spec_step_test.go`: test project chỉ có `ExecutionProviders` (không có `CLIEngineConfig`) → `resolveConfig` trả đúng config từ `CLIProfiles`; test quota-exceeded → `SetCooldown` được gọi đúng `credID`; test project cũ (chỉ `CLIEngineConfig`) → hành vi không đổi.
- Satisfies: REQ-001

### Task 1.2: `worker.go` chọn workflow shape qua Router
- [x] Thêm `shouldUseCLISpecFirstWorkflow(ctx, task, project) bool` vào `execution_router.go`.
- [x] `worker.go`: thay đoạn `cliengine.ResolveEngine(...)` bằng gọi hàm mới; giữ nguyên việc lấy `project` 1 lần (đang lấy 2 lần rải rác — gộp lại thành 1 lần lấy, dùng cho cả `includeCrossReview` lẫn hàm mới).
- [x] `worker_test.go`: test project chỉ có `ExecutionProviders` với 1 candidate cli active → chọn `CLISpecFirstWorkflow`; test tất cả candidate cli không khả dụng → rơi về DAG; test project cũ (`ExecutionEngine="cli"`, `ExecutionProviders` rỗng) → hành vi không đổi.
- Satisfies: REQ-002

### Task 1.3: `TaskService.validateTaskEngineOverride` check `ExecutionProviders`
- [x] Sửa `internal/service/task.go` theo design.md — check `ExecutionProviders` trước, fallback `CLIEngineConfig` khi rỗng.
- [x] `task_test.go`: test project chỉ có `ExecutionProviders` với 1 entry cli enabled → tạo/update task với `execution_engine="cli"` thành công; test không entry cli nào enabled + `CLIEngineConfig` rỗng → vẫn 400; test project cũ → hành vi không đổi.
- Satisfies: REQ-003

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-001 | 1.1 |
| REQ-002 | 1.2 |
| REQ-003 | 1.3 |
